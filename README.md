# CanaraLink — загрузки

Клиент для подключения к VPN-серверам по подписке от любого провайдера.

Приложение не продаёт доступ: подписку вы добавляете сами — по ссылке, из буфера обмена, по
QR-коду или вручную. Поддерживаются VLESS (в том числе Reality), VMess, Trojan, Shadowsocks,
WireGuard и AmneziaWG. Есть раздельное туннелирование по программам, «Умный режим», локальная
сеть напрямую, защита от утечек и значок в трее.

## Windows 10/11

| Архитектура | Установщик |
|---|---|
| x64 (Intel, AMD) | [CanaraLink-x64-setup.exe](https://github.com/canaralink/canaralink-desktop/releases/latest/download/CanaraLink-x64-setup.exe) |
| ARM64 (Snapdragon) | [CanaraLink-arm64-setup.exe](https://github.com/canaralink/canaralink-desktop/releases/latest/download/CanaraLink-arm64-setup.exe) |

Все версии — во вкладке [Releases](https://github.com/canaralink/canaralink-desktop/releases).

Установщик пока без цифровой подписи: Windows покажет «Система Windows защитила ваш компьютер» —
нажмите «Подробнее» → «Выполнить в любом случае».

## Сторонний код

Изменённые файлы Xray-core (MPL-2.0) — в папке [`xray-patch`](xray-patch).

