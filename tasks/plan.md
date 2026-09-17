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
