#!/usr/bin/env bash
# agys_mod installer
set -euo pipefail

DEST_DIR="${HOME}/.local/bin"
mkdir -p "$DEST_DIR"

echo "⚡ Building agys_mod binary..."
go build -ldflags="-s -w" -o "$DEST_DIR/agys" main.go

echo "⚡ Installing auxiliary tools..."
cp scripts/agys-sync.sh "$DEST_DIR/agys-sync"
chmod +x "$DEST_DIR/agys-sync"

cp scripts/agy-quota.py "$DEST_DIR/agy-quota"
chmod +x "$DEST_DIR/agy-quota"
ln -sf "$DEST_DIR/agy-quota" "$DEST_DIR/agyq"

cp scripts/notify-sound.sh "$DEST_DIR/notify-sound.sh"
chmod +x "$DEST_DIR/notify-sound.sh"

# Link notification script into Gemini/Antigravity hooks path if directory exists
GEMINI_BIN="${HOME}/.gemini/config/bin"
if [ -d "${HOME}/.gemini/config" ]; then
  mkdir -p "$GEMINI_BIN"
  ln -sf "$DEST_DIR/notify-sound.sh" "$GEMINI_BIN/notify-sound.sh"
fi

echo "✨ Successfully installed agys_mod to $DEST_DIR"
echo ""
echo "Recommended shell configuration (add to ~/.zshrc or ~/.bashrc):"
echo "---------------------------------------------------------------"
cat <<'EOF'
# Antigravity Multi-Account (agys_mod)
agys() {
  "${HOME}/.local/bin/agys-sync" --quiet 2>/dev/null
  command agys "$@"
}
agy() { agys run "$@"; }
agyq() { "${HOME}/.local/bin/agy-quota" "$@"; }
alias agy1="agys use agy1 && agys run agy1"
alias agy2="agys use agy2 && agys run agy2"
alias agy3="agys use agy3 && agys run agy3"
alias agy4="agys use agy4 && agys run agy4"
alias use1="agys use agy1"
alias use2="agys use agy2"
alias use3="agys use agy3"
alias use4="agys use agy4"
EOF
echo "---------------------------------------------------------------"
