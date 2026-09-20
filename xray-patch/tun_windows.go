//go:build windows

// Патч CanaraLink к github.com/xtls/xray-core/proxy/tun/tun_windows.go (v1.260327.0),
// подключается при сборке через -overlay (см. core/build.sh). Отличия от оригинала:
//
//   - Close безопасен при работающем цикле чтения: ожидание пакетов прерывается событием
//     закрытия (как в wireguard-go), а чтение и запись не трогают сессию после End —
//     иначе служба падала бы с нарушением доступа к памяти при каждом отключении;
//   - пакет из кольца wintun копируется и сразу возвращается: gVisor держит буферы
//     сколько угодно долго, и освобождение после End было бы тем же use-after-free.
//
// Оригинальный Xray рассчитан на отдельный процесс, где адаптер умирает вместе с ним; у нас
// ядро живёт в службе и подключается многократно.

package tun

import (
	"crypto/md5"
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

//go:linkname procyield runtime.procyield
func procyield(cycles uint32)

// WindowsTun is an object that handles tun network interface on Windows
type WindowsTun struct {
	options    TunOptions
	adapter    *wintun.Adapter
	session    wintun.Session
	readWait   windows.Handle
	closeEvent windows.Handle
	MTU        uint32

	// mu: чтение и запись — под RLock, Close — под Lock; после closed сессию не трогаем.
	mu     sync.RWMutex
	closed bool
}

var errTunClosed = errors.New("tun closed")

// WindowsTun implements Tun
var _ Tun = (*WindowsTun)(nil)

// WindowsTun implements GVisorTun
var _ GVisorTun = (*WindowsTun)(nil)

// WindowsTun implements GVisorDevice
var _ GVisorDevice = (*WindowsTun)(nil)

// NewTun creates a Wintun interface with the given name. Should a Wintun
// interface with the same name exist, it tried to be reused.
func NewTun(options TunOptions) (Tun, error) {
	adapter, err := open(options.Name)
	if err != nil {
		return nil, err
	}

	session, err := adapter.StartSession(0x800000)
	if err != nil {
		_ = adapter.Close()
		return nil, err
	}

	closeEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		session.End()
		_ = adapter.Close()
		return nil, err
	}

	tun := &WindowsTun{
		options:    options,
		adapter:    adapter,
		session:    session,
		readWait:   session.ReadWaitEvent(),
		closeEvent: closeEvent,
		MTU:        wintun.PacketSizeMax,
	}

	return tun, nil
}

func open(name string) (*wintun.Adapter, error) {
	// generate a deterministic GUID from the adapter name
	id := md5.Sum([]byte(name))
	guid := (*windows.GUID)(unsafe.Pointer(&id[0]))
	// try to open existing adapter by name
	adapter, err := wintun.OpenAdapter(name)
	if err == nil {
		return adapter, nil
	}
	// try to create adapter anew
	adapter, err = wintun.CreateAdapter(name, "Xray", guid)
	if err == nil {
		return adapter, nil
	}
	return nil, err
}

func (t *WindowsTun) Start() error {
	return nil
}

// Close завершает сессию и удаляет адаптер. Повторный вызов ничего не делает.
func (t *WindowsTun) Close() error {
	_ = windows.SetEvent(t.closeEvent)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	t.session.End()
	err := t.adapter.Close()
	_ = windows.CloseHandle(t.closeEvent)
	return err
}

// WritePacket implements GVisorDevice method to write one packet to the tun device
func (t *WindowsTun) WritePacket(packetBuffer *stack.PacketBuffer) tcpip.Error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return &tcpip.ErrAborted{}
	}

	packet, err := t.session.AllocateSendPacket(packetBuffer.Size())
	if err != nil {
		return &tcpip.ErrAborted{}
	}

	var index int
	for _, packetElement := range packetBuffer.AsSlices() {
		index += copy(packet[index:], packetElement)
	}

	t.session.SendPacket(packet)

	return nil
}

// ReadPacket implements GVisorDevice method to read one packet from the tun device
// It is expected that the method will not block, rather return ErrQueueEmpty when there is nothing on the line,
// which will make the stack call Wait which should implement desired push-back
func (t *WindowsTun) ReadPacket() (byte, *stack.PacketBuffer, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return 0, nil, errTunClosed
	}

	packet, err := t.session.ReceivePacket()
	if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
		return 0, nil, ErrQueueEmpty
	}
	if err != nil {
		return 0, nil, err
	}

	data := make([]byte, len(packet))
	copy(data, packet)
	t.session.ReleaseReceivePacket(packet)

	version := data[0] >> 4
	packetBuffer := buffer.MakeWithView(buffer.NewViewWithData(data))
	return version, stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload:           packetBuffer,
		IsForwardedPacket: true,
	}), nil
}

// Wait ждёт пакет или закрытие — без второго события цикл чтения висел бы вечно.
func (t *WindowsTun) Wait() {
	procyield(1)
	_, _ = windows.WaitForMultipleObjects([]windows.Handle{t.readWait, t.closeEvent}, false, windows.INFINITE)
}

func (t *WindowsTun) newEndpoint() (stack.LinkEndpoint, error) {
	return &LinkEndpoint{deviceMTU: t.options.MTU, device: t}, nil
}
