#!/usr/bin/env bash
# Antigravity sound & terminal attention notification handler
set -e

EVENT="${1:-finish}"
MSG="${2:-}"

case "$EVENT" in
  action|ask)
    SOUND="/System/Library/Sounds/Ping.aiff"
    SOUND_NAME="Ping"
    TITLE="Antigravity — Action Required"
    TEXT="${MSG:-Agent requested your choice or confirmation}"
    DECISION_JSON='{"decision": "allow"}'
    ;;
  finish|stop|*)
    SOUND="/System/Library/Sounds/Glass.aiff"
    SOUND_NAME="Glass"
    TITLE="Antigravity — Task Complete"
    TEXT="${MSG:-Agent finished working on task}"
    DECISION_JSON='{"decision": "allow"}'
    ;;
esac

# Debounce rapid triggers within 2 seconds for the same event
LOCK_FILE="${TMPDIR:-/tmp}/agys-notify-last-${EVENT}"
NOW=$(date +%s)
LAST_TIME=$(cat "$LOCK_FILE" 2>/dev/null || echo 0)
if [ $((NOW - LAST_TIME)) -lt 2 ]; then
  echo "$DECISION_JSON"
  exit 0
fi
echo "$NOW" > "$LOCK_FILE"

# 1. Play audio (macOS afplay or Linux paplay/aplay)
if command -v afplay >/dev/null 2>&1 && [ -f "$SOUND" ]; then
  afplay -v 0.65 "$SOUND" >/dev/null 2>&1 &
elif command -v paplay >/dev/null 2>&1 && [ -f "/usr/share/sounds/freedesktop/stereo/complete.oga" ]; then
  paplay "/usr/share/sounds/freedesktop/stereo/complete.oga" >/dev/null 2>&1 &
fi

# 2. Trigger Ghostty/terminal bell (\a) and attention OSC sequence on current tty
CURRENT_TTY=""
cur_pid=$$
for _ in {1..8}; do
  cur_pid=$(ps -p "$cur_pid" -o ppid= 2>/dev/null | tr -d ' ')
  [ -z "$cur_pid" ] && break
  tty_candidate=$(ps -p "$cur_pid" -o tty= 2>/dev/null | tr -d ' ')
  if [ -n "$tty_candidate" ] && [ "$tty_candidate" != "?" ] && [ -w "/dev/$tty_candidate" ]; then
    CURRENT_TTY="$tty_candidate"
    break
  fi
done

if [ -n "$CURRENT_TTY" ] && [ -w "/dev/$CURRENT_TTY" ]; then
  (printf '\a\033]777;notify;%s;%s\033\\' "$TITLE" "$TEXT" > "/dev/$CURRENT_TTY" 2>/dev/null || true) &
elif [ -w "/dev/tty" ]; then
  (printf '\a\033]777;notify;%s;%s\033\\' "$TITLE" "$TEXT" > "/dev/tty" 2>/dev/null || true) &
fi

# 3. Trigger Desktop Notification banner (macOS or Linux notify-send)
if command -v osascript >/dev/null 2>&1; then
  (osascript -e "display notification \"$TEXT\" with title \"$TITLE\"" >/dev/null 2>&1 || true) &
elif command -v notify-send >/dev/null 2>&1; then
  (notify-send "$TITLE" "$TEXT" >/dev/null 2>&1 || true) &
fi

# 4. Output required JSON contract to stdout
echo "$DECISION_JSON"
