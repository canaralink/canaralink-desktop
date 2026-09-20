// Дополнение CanaraLink к github.com/xtls/xray-core/proxy/tun (подключается через -overlay).
// В оригинале у Handler нет Close: адаптер wintun жил до конца процесса, и в службе, которая
// подключается много раз, адаптеры и сессии копились бы.

package tun

// Close гасит стек gVisor и закрывает TUN (на Windows — удаляет адаптер).
func (t *Handler) Close() error {
	if t.stack == nil {
		return nil
	}
	err := t.stack.Close()
	if s, ok := t.stack.(*stackGVisor); ok {
		if c, ok := s.tun.(Tun); ok {
			if e := c.Close(); err == nil {
				err = e
			}
		}
	}
	t.stack = nil
	return err
}
