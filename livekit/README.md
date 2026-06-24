# LiveKit — медиасервер для прямых эфиров

Эфиры идут через self-hosted LiveKit (SFU): вещатель публикует поток **один раз**
на сервер, сервер раздаёт его всем зрителям. Бэкенд (Go) только выдаёт токены
доступа к комнатам и хранит список эфиров (`live_streams`).

## Локально — БЕЗ докера (бинарник)

1. Скачать бинарник `livekit-server`:
   - Windows: https://github.com/livekit/livekit/releases (распаковать `livekit-server.exe`)
   - или scoop/winget, или `curl -sSL https://get.livekit.io | bash` на mac/linux.
2. Запустить из папки `backend`:
   ```
   livekit-server --config ./livekit/livekit.yaml
   ```
   Поднимется на `:7880` (ws) и `:7881` (rtc tcp) + udp 50000-50100.
3. В `backend/.env` прописать (ключи = из `livekit.yaml`):
   ```
   LIVEKIT_URL=ws://172.20.10.3:7880
   LIVEKIT_API_KEY=seeu_dev_key
   LIVEKIT_API_SECRET=seeu_dev_secret_change_me_0123456789abcdef
   ```
   `172.20.10.3` — LAN-IP ноута (как в `test_old/lib/core/config/app_config.dart`).
   Телефон и ноут — в одной Wi-Fi сети.

Проверка, что сервер жив: `curl http://172.20.10.3:7880` → отдаёт `OK`.

## Продакшен — docker

См. `backend/docker-compose.prod.yml`. Открыть UDP-порты, поставить TLS-прокси
для `wss://`, ключи и `NODE_IP` передать через окружение.
