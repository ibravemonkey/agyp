# Спецификация: Команда `agys plugin`

## Обзор
Добавление набора подкоманд `agys plugin` (`install`, `list`, `uninstall`) с поддержкой флага `--all` (`-a`). Команда управляет плагинами напрямую в изолированных окружениях профилей без накладных расходов на инициализацию интерактивных диалогов CLI.

## Синтаксис команд
- `agys plugin install <target> [profile_name] [--all/-a]`
- `agys plugin list [profile_name] [--all/-a]`
- `agys plugin uninstall <name> [profile_name] [--all/-a]`

## Поведение и требования
1. **Облегченное выполнение**:
   - Выполняет `agy plugin ...`, устанавливая `HOME=<profileDir>` без запуска трекинга сессий, синхронизации keychain или интерактивных баннеров.
2. **Компактный формат вывода**:
   - При передаче `--all` / `-a` выводит компактные индикаторы прогресса по каждому профилю:
     ```text
     [1/6] tram520      ✔ superpowers installed
     [2/6] quaywin      ✔ superpowers installed
     ...
     [agys] Plugin superpowers processed across 6 profiles.
     ```
3. **Поддержка профиля по умолчанию**:
   - Если `--all` не указан и имя профиля не передано, используется активный профиль по умолчанию (через `profile.GetCurrent()`).
4. **Псевдонимы**:
   - Регистрация `agys plugins` как псевдонима для `agys plugin`.

## Изменения в файлах
- Создан `cmd/plugin.go` со структурами команд Cobra `pluginCmd`, `pluginInstallCmd`, `pluginListCmd` и `pluginUninstallCmd`.
- Создан `cmd/plugin_test.go` с модульными тестами для парсинга флагов и регистрации команд.
- Обновлен `README.md` с примерами использования.
