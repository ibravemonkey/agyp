#!/usr/bin/env python3
import json
import os
import sys
import subprocess

# ANSI Colors
RST = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[90m"
GREEN = "\033[92m"
B_GREEN = "\033[1;92m"
YELLOW = "\033[93m"
B_YELLOW = "\033[1;93m"
RED = "\033[91m"
B_RED = "\033[1;91m"
CYAN = "\033[96m"
B_CYAN = "\033[1;96m"
WHITE = "\033[97m"
B_WHITE = "\033[1;97m"

def bar(pct, width=12):
    filled = int(round(pct * width))
    filled = max(0, min(width, filled))
    empty = width - filled
    if pct >= 0.6:
        color = GREEN
    elif pct >= 0.25:
        color = YELLOW
    else:
        color = RED
    return f"{color}{'█' * filled}{DIM}{'░' * empty}{RST}"

def color_pct(pct):
    p = int(round(pct * 100))
    if pct >= 0.6:
        return f"{B_GREEN}{p:>3}%{RST}"
    elif pct >= 0.25:
        return f"{B_YELLOW}{p:>3}%{RST}"
    else:
        return f"{B_RED}{p:>3}%{RST}"

def main():
    args = sys.argv[1:]
    
    # Pass through raw or json flags
    if "-r" in args or "--raw" in args:
        raw_args = [a for a in args if a not in ("-r", "--raw")]
        os.execvp("agys", ["agys", "q"] + raw_args)
    if "-j" in args or "--json" in args or "-h" in args or "--help" in args:
        os.execvp("agys", ["agys", "q"] + args)

    target_profile = None
    for a in args:
        if not a.startswith("-"):
            target_profile = a
            break

    # Read active default profile
    current = ""
    current_file = os.path.expanduser("~/.agys/current")
    if os.path.isfile(current_file):
        try:
            with open(current_file) as f:
                current = f.read().strip()
        except Exception:
            pass

    # Fetch quota JSON from agyp binary
    agyp_bin = os.path.expanduser("~/.local/bin/agyp")
    if not os.path.isfile(agyp_bin):
        agyp_bin = "agyp"
    cmd = [agyp_bin, "q", "--json"]
    if target_profile:
        cmd.append(target_profile)

    try:
        raw_json = subprocess.check_output(cmd, stderr=subprocess.PIPE).decode("utf-8")
        data = json.loads(raw_json)
    except subprocess.CalledProcessError as e:
        sys.stderr.write(e.stderr.decode("utf-8"))
        sys.exit(e.returncode)
    except Exception as e:
        sys.stderr.write(f"Error fetching quotas: {e}\n")
        sys.exit(1)

    # If single object returned instead of list
    if isinstance(data, dict):
        data = [data]

    if not data:
        print(f"{DIM}No profile quota data available.{RST}")
        return

    active_tag = f"{B_GREEN}{current}{RST}" if current else f"{YELLOW}none{RST}"
    print(f"\n{B_WHITE}⚡ Antigravity Multi-Account Quotas{RST}  {DIM}(active: {active_tag}{DIM}){RST}\n")

    for p in data:
        name = p.get("profileName", "")
        email = p.get("email", "unknown")
        is_active = (name == current)

        g_5h_pct = 1.0
        g_5h_reset = ""
        g_wk_pct = 1.0
        g_wk_reset = ""
        c_5h_pct = 1.0
        c_wk_pct = 1.0

        for g in p.get("quota", {}).get("groups", []):
            dname = g.get("displayName", "")
            if "Gemini" in dname:
                for b in g.get("buckets", []):
                    w = b.get("window", "")
                    pct = float(b.get("remainingFraction", 1.0))
                    desc = b.get("description", "")
                    reset = ""
                    if "refresh in " in desc:
                        reset = desc.split("refresh in ")[-1].rstrip(".")

                    if w == "5h":
                        g_5h_pct = pct
                        g_5h_reset = reset
                    elif w == "weekly":
                        g_wk_pct = pct
                        g_wk_reset = reset

            elif "Claude" in dname or "GPT" in dname:
                for b in g.get("buckets", []):
                    w = b.get("window", "")
                    pct = float(b.get("remainingFraction", 1.0))
                    if w == "5h":
                        c_5h_pct = pct
                    elif w == "weekly":
                        c_wk_pct = pct

        if is_active:
            status = f"{B_GREEN}● ACTIVE{RST}"
            pname = f"{B_GREEN}{name:<5}{RST}"
        else:
            status = f"{DIM}○ idle  {RST}"
            pname = f"{B_WHITE}{name:<5}{RST}"

        reset_5h = f"{DIM}(resets in {g_5h_reset}){RST}" if g_5h_reset else f"{GREEN}(ready){RST}"
        reset_wk = f"{DIM}(resets in {g_wk_reset}){RST}" if g_wk_reset else f"{GREEN}(ready){RST}"

        print(f" {status}  {pname} {DIM}│{RST} {WHITE}{email}{RST}")
        print(f"   {DIM}├─ Gemini 5H:{RST}  {bar(g_5h_pct, 12)} {color_pct(g_5h_pct)}  {reset_5h}")
        print(f"   {DIM}├─ Weekly:   {RST}  {bar(g_wk_pct, 12)} {color_pct(g_wk_pct)}  {reset_wk}")
        print(f"   {DIM}└─ 3P/Claude:{RST}  {bar(c_5h_pct, 6)} {color_pct(c_5h_pct)}  {DIM}Weekly:{RST} {bar(c_wk_pct, 6)} {color_pct(c_wk_pct)}")
        print()

if __name__ == "__main__":
    main()
