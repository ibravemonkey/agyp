#!/usr/bin/env bash
# agys_mod installer
set -euo pipefail

DEST_DIR="${HOME}/.local/bin"
mkdir -p "$DEST_DIR"

echo "⚡ Сборка бинарника agys_mod..."
go build -ldflags="-s -w" -o "$DEST_DIR/agys" main.go

echo "⚡ Установка вспомогательных утилит..."
cp scripts/agys-sync.sh "$DEST_DIR/agys-sync"
chmod +x "$DEST_DIR/agys-sync"

cp scripts/notify-sound.sh "$DEST_DIR/notify-sound.sh"
chmod +x "$DEST_DIR/notify-sound.sh"

# Link notification script into Gemini/Antigravity hooks path if directory exists
GEMINI_BIN="${HOME}/.gemini/config/bin"
if [ -d "${HOME}/.gemini/config" ]; then
  mkdir -p "$GEMINI_BIN"
  ln -sf "$DEST_DIR/notify-sound.sh" "$GEMINI_BIN/notify-sound.sh"
fi

echo "⚙️  Автоматическая настройка оболочки..."
"$DEST_DIR/agys" setup-shell --bin-dir "$DEST_DIR"

echo ""
echo "✨ Установка agys_mod успешно завершена!"
echo ""
echo "Следующие шаги:"
echo "  1. Примените изменения в текущем окне: source ~/.zshrc (или ~/.bashrc)"
echo "  2. Подключите аккаунты:               agys add agy1 (и agys add agy2)"
echo "  3. Проверьте квоты всех аккаунтов:    agyq"
echo "  4. Начните работу в Antigravity CLI:   agy1 или agy2"
