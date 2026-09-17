# План реализации: `agys`

## Архитектура и структура
1. `pkg/profile`: Доменная логика профилей (разрешение путей, проверка каталогов, валидация, листинг, удаление, обертка над выполнением команд).
2. `internal/`: Сервисные слои с интерфейсами (`runner`, `doctor`, `selector`, `sshproxy`, `gitops`).
3. `cmd/`: Тонкие обработчики подкоманд Cobra (`root`, `add`, `list`, `delete`, `run`, `doctor`, `commit`, `ssh` и др.).
4. `main.go`: Точка входа приложения, вызывающая `cmd.Execute()`.
5. Релиз и CI/CD: `.goreleaser.yaml` и `.github/workflows/release.yml`.
6. Инсталлятор: скрипт `install.sh`.

## Последовательные этапы
1. Инициализация модуля Go (`go.mod`) и зависимостей Cobra.
2. Реализация пакета `pkg/profile`.
3. Реализация пакетов `cmd/` и `internal/`.
4. Создание `main.go` и верификация локальной сборки CLI.
5. Настройка GoReleaser и GitHub Actions.
6. Разработка скрипта установки `install.sh`.

## Расширение 2: Умная очистка (clean/prune) и шифрование профилей (export/import --encrypt)
### 1. Умная очистка по TTL (`pkg/profile/clean.go`, `cmd/clean.go`):
- Парсер TTL: поддержка `14d`, `7d`, `24h`, `30m` и стандартных duration Go.
- Анализ размера файлов и сессий в `brain/<conv_id>/` по `transcript.jsonl` mtime.
- Защита от полного удаления: сохранение минимум `--keep-last` сессий (по умолчанию 5).
- Очистка временных кэшей: `ide-data/logs`, `ide-data/blob_storage`, `Crashpad`, `crashes`, `Caches`, `.cache`, stale сокетов/локов.
- Синхронизация `history.jsonl`: удаление записей удаленных сессий.
- Режим `--dry-run` (`-n`) для безопасного превью освобождаемого места.
- Команда `agyp clean` (алиас `prune`) с флагами `--ttl`, `--keep-last`, `--all`, `--dry-run`, `--cache-only`, `--sessions-only`.

### 2. Шифрование профилей при экспорте (`pkg/profile/crypto.go`, `cmd/export.go`, `cmd/import.go`):
- Стандарт: AES-256-GCM + PBKDF2-HMAC-SHA256 (100k итераций, 16B salt, 12B nonce).
- Контейнер `AGYP_ENC\x01` с проверкой целостности через GCM auth tag.
- Потоковое шифрование и расшифровка в памяти без сохранения промежуточных открытых файлов.
- Интерактивный скрытый ввод пароля через `term.ReadPassword` или флаг `--password` / env `AGYP_ENCRYPTION_KEY`.
- Авто-определение зашифрованного архива при `agyp import`.
