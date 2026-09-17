# agys_mod ⚡

> **Enhanced multi-profile isolation, real-time telemetry statusline, and workspace automation for Google Antigravity CLI (`agy`).**

`agys_mod` is an upgraded fork/extension of `agys`. It provides effortless profile switching, multi-account quota monitoring, zero-pollution workspace sandboxing, and an enhanced statusline with Git branch tracking, agent lifecycle indicators, and sound/terminal notifications.

---

## ✨ Features & Enhancements

### 1. 🖥️ Enhanced Real-Time Statusline
Rendered directly in your terminal footer during `agy` / Antigravity CLI sessions:
```text
[agy1] · 📦 my-project ·  main ·  Working · 12% ctx · gemini-3.7-flash (high) · $0.0024 · 5H: 84% (1h14m) · Wk: 72%
```
- **Profile tag:** High-contrast cyan profile badge (`[agy1]`).
- **Workspace detection:** Identifies current project directory / workspace name (`📦 my-project`).
- **Git branch detection:** Automatically walks directory tree to find active Git branch or detached HEAD (` main`). Works across regular repos and Git worktrees.
- **Agent state indicator:** Real-time visual indicator with Nerd Font glyphs:
  - ` Working` (yellow) — Agent actively processing or generating code
  - `󰧑 Thinking` (cyan) — Model reasoning / planning
  - ` Waiting` (yellow) — Awaiting user input or confirmation
  - ` Done` (green) — Task complete
  - ` Idle` (green) — Ready for next prompt
- **Context window telemetry:** Active token consumption (`% ctx`) with adaptive green/yellow/red color thresholds.
- **Model & effort:** Active model + reasoning effort level.
- **Real-time quotas:** 5-hour and weekly Gemini / Claude quota tracking with compact countdown timers.

### 2. 🔔 Completion Audio & Desktop Notifications
- When an agent finishes a task (`Working`/`Thinking` ➔ `Done`), `agys_mod` triggers:
  - Soft native audio feedback (e.g. macOS `Glass.aiff` / Linux system audio).
  - Terminal attention bell (`\a` + OSC notification sequence supported by Ghostty, iTerm2, WezTerm, Kitty).
  - Desktop notification banner.
- Fully portable: dynamically discovers `notify-sound.sh` via environment variable `$AGYS_NOTIFY_SOUND_SCRIPT`, `~/.gemini/config/bin`, `~/.local/bin`, or system `$PATH`.

### 3. 🔄 Universal Profile Sync (`agys-sync`)
- Seamlessly mirrors shared configs across all profiles (`~/.agys/profiles/*`):
  - Shell toolchains (`.local`, `.cargo`, `.ssh`, `.gitconfig`, `.zsh`, `.oh-my-zsh`, `.nvm`, etc.)
  - Rules, skills, compass directives, hooks, and MCP servers (`~/.gemini/config/*`).
  - No duplicated tool installations or broken PATHs inside isolated profile homes.

### 4. 📊 Visual Quota Dashboard (`agy-quota` / `agyq`)
Interactive terminal dashboard with colored progress bars:
```text
⚡ Antigravity Multi-Account Quotas  (active: agy1)

 ● ACTIVE  agy1  │ user1@gmail.com
   ├─ Gemini 5H:  ██████████░░  84%  (resets in 1h 14m)
   ├─ Weekly:     ████████░░░░  72%  (resets in 3d 8h)
   └─ 3P/Claude:  ████░░  65%  Weekly: ██████ 100%

 ○ idle    agy2  │ user2@gmail.com
   ├─ Gemini 5H:  ████████████ 100%  (ready)
   ├─ Weekly:     ████████████ 100%  (ready)
   └─ 3P/Claude:  ██████ 100%  Weekly: ██████ 100%
```

---

## 🚀 Installation

### Automated Install
Clone or copy this repository, then run the installer:
```bash
./install.sh
```
This builds the `agys` binary, installs all helper tools into `~/.local/bin`, and creates necessary symlinks.

### Manual Build
```bash
go build -ldflags="-s -w" -o ~/.local/bin/agys main.go
cp scripts/agys-sync.sh ~/.local/bin/agys-sync && chmod +x ~/.local/bin/agys-sync
cp scripts/agy-quota.py ~/.local/bin/agy-quota && chmod +x ~/.local/bin/agy-quota
ln -sf ~/.local/bin/agy-quota ~/.local/bin/agyq
cp scripts/notify-sound.sh ~/.local/bin/notify-sound.sh && chmod +x ~/.local/bin/notify-sound.sh
```

---

## ⚙️ Shell Configuration

Add the following to your `~/.zshrc` or `~/.bashrc`:

```bash
# Antigravity Multi-Account (agys_mod)
agys() {
  "${HOME}/.local/bin/agys-sync" --quiet 2>/dev/null
  command agys "$@"
}

# Run Antigravity CLI through agys
agy()  { agys run "$@"; }

# Visual Quota overview
agyq() { "${HOME}/.local/bin/agy-quota" "$@"; }

# Direct Profile Launchers
alias agy1="agys use agy1 && agys run agy1"
alias agy2="agys use agy2 && agys run agy2"
alias agy3="agys use agy3 && agys run agy3"
alias agy4="agys use agy4 && agys run agy4"

# Fast Profile Switching
alias use1="agys use agy1"
alias use2="agys use agy2"
alias use3="agys use agy3"
alias use4="agys use agy4"
```

Then reload your shell:
```bash
source ~/.zshrc
```

---

## 🛠️ Usage with `agy` (Antigravity CLI)

### 1. Add Profiles
Add your Google accounts as distinct isolated profiles:
```bash
agys add agy1
agys add agy2
agys add agy3
agys add agy4
```

### 2. Inspect Quotas
```bash
agyq
```

### 3. Run Antigravity CLI
Run with the currently active profile:
```bash
agy
```
Or launch directly with a specific profile:
```bash
agy1
# or
agys run agy2
```

### 4. Automatic Profile Selection by Quota
Run with the profile that currently has the best remaining Gemini 5-hour quota:
```bash
agys auto
```

### 5. Synchronize Profiles
Manually trigger profile symlinks & skills synchronization:
```bash
agys-sync
```

---

## 🌍 Portability & Architecture

- **No Hardcoded Paths:** All user directories resolve dynamically via `$HOME` / `os.UserHomeDir()`.
- **Cross-Platform Audio & Alerts:** macOS uses `afplay` and `osascript`; Linux systems fall back to `paplay` / `notify-send` and standard terminal bell sequences (`\a`, OSC 777).
- **Graceful Degradation:** If audio or Git repositories are unavailable, the statusline displays cleanly without errors.
- **Zero Pollution:** Each profile's state, auth tokens, and session context live completely isolated in `~/.agys/profiles/<name>/`.

---

## 📜 License

MIT License. See [LICENSE](LICENSE) for details.
