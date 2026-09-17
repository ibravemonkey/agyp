#!/usr/bin/env bash
# agys-sync: Synchronizes skills, hooks, tools, and shared configurations across all agys profiles.
set -euo pipefail

BASE_HOME="${HOME:-$(eval echo ~${USER:-})}"
PROFILES_DIR="$BASE_HOME/.agys/profiles"

if [ ! -d "$PROFILES_DIR" ]; then
  exit 0
fi

QUIET=false
if [ "${1:-}" = "--quiet" ] || [ "${1:-}" = "-q" ]; then
  QUIET=true
fi

log() {
  if [ "$QUIET" = false ]; then
    echo "$@"
  fi
}

log "⚡ Synchronizing agys profiles with base environment..."

for profile in "$PROFILES_DIR"/*; do
  [ -d "$profile" ] || continue
  pname=$(basename "$profile")

  # 1. Shell & CLI tools (.local, .cargo, .ssh, .gitconfig, oh-my-zsh, nvm, etc.)
  [ -d "$BASE_HOME/.local" ] && [ ! -e "$profile/.local" ] && ln -s "$BASE_HOME/.local" "$profile/.local"
  [ -d "$BASE_HOME/.cargo" ] && [ ! -e "$profile/.cargo" ] && ln -s "$BASE_HOME/.cargo" "$profile/.cargo"
  [ -d "$BASE_HOME/.ssh" ] && [ ! -e "$profile/.ssh" ] && ln -s "$BASE_HOME/.ssh" "$profile/.ssh"
  [ -f "$BASE_HOME/.gitconfig" ] && [ ! -e "$profile/.gitconfig" ] && ln -sf "$BASE_HOME/.gitconfig" "$profile/.gitconfig"
  [ -d "$BASE_HOME/.oh-my-zsh" ] && [ ! -e "$profile/.oh-my-zsh" ] && ln -s "$BASE_HOME/.oh-my-zsh" "$profile/.oh-my-zsh"
  [ -f "$BASE_HOME/.p10k.zsh" ] && [ ! -e "$profile/.p10k.zsh" ] && ln -sf "$BASE_HOME/.p10k.zsh" "$profile/.p10k.zsh"
  [ -d "$BASE_HOME/.zsh" ] && [ ! -e "$profile/.zsh" ] && ln -s "$BASE_HOME/.zsh" "$profile/.zsh"
  [ -d "$BASE_HOME/.nvm" ] && [ ! -e "$profile/.nvm" ] && ln -s "$BASE_HOME/.nvm" "$profile/.nvm"

  # 2. Base .gemini directories
  mkdir -p "$profile/.gemini/config" "$profile/.gemini/antigravity-cli"

  # 3. Directives (GEMINI.md)
  if [ -f "$BASE_HOME/.gemini/GEMINI.md" ]; then
    ln -sf "$BASE_HOME/.gemini/GEMINI.md" "$profile/.gemini/GEMINI.md"
  fi

  # 4. Config items (.gemini/config/...)
  # bin
  if [ -d "$profile/.gemini/config/bin" ] && [ ! -L "$profile/.gemini/config/bin" ]; then
    rm -rf "$profile/.gemini/config/bin"
  fi
  if [ -d "$BASE_HOME/.gemini/config/bin" ]; then
    ln -sfn "$BASE_HOME/.gemini/config/bin" "$profile/.gemini/config/bin"
  fi

  # skill-compass
  if [ -d "$profile/.gemini/config/skill-compass" ] && [ ! -L "$profile/.gemini/config/skill-compass" ]; then
    rm -rf "$profile/.gemini/config/skill-compass"
  fi
  if [ -d "$BASE_HOME/.gemini/config/skill-compass" ]; then
    ln -sfn "$BASE_HOME/.gemini/config/skill-compass" "$profile/.gemini/config/skill-compass"
  fi

  # hooks.json
  if [ -f "$BASE_HOME/.gemini/config/hooks.json" ]; then
    ln -sf "$BASE_HOME/.gemini/config/hooks.json" "$profile/.gemini/config/hooks.json"
  fi

  # rules
  if [ -d "$profile/.gemini/config/rules" ] && [ ! -L "$profile/.gemini/config/rules" ]; then
    rm -rf "$profile/.gemini/config/rules"
  fi
  if [ -d "$BASE_HOME/.gemini/config/rules" ]; then
    ln -sfn "$BASE_HOME/.gemini/config/rules" "$profile/.gemini/config/rules"
  fi

  # plugins
  if [ -d "$profile/.gemini/config/plugins" ] && [ ! -L "$profile/.gemini/config/plugins" ]; then
    rm -rf "$profile/.gemini/config/plugins"
  fi
  if [ -d "$BASE_HOME/.gemini/config/plugins" ]; then
    ln -sfn "$BASE_HOME/.gemini/config/plugins" "$profile/.gemini/config/plugins"
  fi

  # skills
  if [ -d "$profile/.gemini/config/skills" ] && [ ! -L "$profile/.gemini/config/skills" ]; then
    rm -rf "$profile/.gemini/config/skills"
  fi
  if [ -d "$BASE_HOME/.gemini/config/skills" ]; then
    ln -sfn "$BASE_HOME/.gemini/config/skills" "$profile/.gemini/config/skills"
  fi

  # JSON configs & MCP
  [ -f "$BASE_HOME/.gemini/config/plugins.json" ] && ln -sf "$BASE_HOME/.gemini/config/plugins.json" "$profile/.gemini/config/plugins.json"
  [ -f "$BASE_HOME/.gemini/config/skills.json" ] && ln -sf "$BASE_HOME/.gemini/config/skills.json" "$profile/.gemini/config/skills.json"
  if [ -f "$BASE_HOME/.gemini/config/mcp_config.json" ]; then
    ln -sf "$BASE_HOME/.gemini/config/mcp_config.json" "$profile/.gemini/config/mcp_config.json"
    ln -sf "$BASE_HOME/.gemini/config/mcp_config.json" "$profile/.gemini/antigravity-cli/mcp_config.json"
  fi

  # 5. Antigravity-cli symlinks
  [ -f "$BASE_HOME/.gemini/config/hooks.json" ] && ln -sf "$BASE_HOME/.gemini/config/hooks.json" "$profile/.gemini/antigravity-cli/hooks.json"
  [ -d "$BASE_HOME/.gemini/config/skills" ] && ln -sfn "$BASE_HOME/.gemini/config/skills" "$profile/.gemini/antigravity-cli/skills"

  log "  ✓ Profile '$pname' synced"
done

log "✨ All profiles successfully linked with skills, rtk, sqz, compass and directives!"
