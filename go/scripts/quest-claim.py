#!/usr/bin/env python3
"""Claim custom Java quest handlers so two agents never port the same one.

A claim is a directory .quest-claims/<id>; mkdir is atomic, so of two agents claiming
the same id exactly one succeeds. `done` ids (registered in game/data/quest_scripts.go,
or marked done by hand) can't be claimed.

  quest-claim.py list [n]          unclaimed, unported handlers (first n)
  quest-claim.py next <who> [n] [--from-end]  atomically claim the next n (default 1) from the lowest
                                   ids (or the highest with --from-end), print them
  quest-claim.py claim <who> <id>  claim one id; exit 1 if it is taken
  quest-claim.py release <id>      give a claim back
  quest-claim.py done <id>         mark ported (after registering it and its tests pass)
  quest-claim.py status            counts, and who holds what

Audit mode: already registered handlers with an open row in QUEST_AUDIT.md. Claims are
.quest-claims/audit-<id>, so they never clash with port claims.
  quest-claim.py audit-next <who> [n]  atomically claim the next n (default 1) open audit ids
  quest-claim.py audit-done <id>       mark the audit of a handler finished (after fixing its rows)
  quest-claim.py audit-status          open / claimed / done counts, and who holds what
"""
import os, re, sys, glob

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
CLAIMS = os.path.join(ROOT, ".quest-claims")
JAVA = os.path.join(os.environ.get("AION_JAVA", os.path.join(ROOT, "..", "java", "AL-Game")), "data/scripts/system/handlers/quest")

def handlers():
    out = {}
    for f in glob.glob(JAVA + "/*/_*.java"):
        m = re.match(r"_(\d+)(.*)\.java", os.path.basename(f))
        if m:
            out[int(m.group(1))] = (os.path.basename(os.path.dirname(f)), m.group(2), f)
    return out

def registered():
    src = open(os.path.join(ROOT, "game/data/quest_scripts.go")).read()
    return {int(x) for x in re.findall(r"\{ID:\s*(\d+),\s*Kind:\s*QuestCustom", src)}

def state(i):
    d = os.path.join(CLAIMS, str(i))
    if not os.path.isdir(d):
        return None
    who = open(os.path.join(d, "who")).read().strip() if os.path.exists(os.path.join(d, "who")) else "?"
    return "done" if os.path.exists(os.path.join(d, "done")) else who

def claim(i, who):
    os.makedirs(CLAIMS, exist_ok=True)
    try:
        os.mkdir(os.path.join(CLAIMS, str(i)))
    except FileExistsError:
        return False
    open(os.path.join(CLAIMS, str(i), "who"), "w").write(who + "\n")
    return True

def free():
    reg, h = registered(), handlers()
    return [i for i in sorted(h) if i not in reg and state(i) is None]

def show(i):
    zone, name, _ = handlers()[i]
    return f"{i}\t{zone}\t{name}"

def audit_open():
    """Quest ids that have an open row (`| <id> | ... | open |`) in QUEST_AUDIT.md, with their violations."""
    rows = {}
    for line in open(os.path.join(ROOT, "QUEST_AUDIT.md")):
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) >= 5 and cells[0].isdigit() and cells[-1] == "open":
            rows.setdefault(int(cells[0]), []).append(cells[2])
    return rows

def audit_state(i):
    return state("audit-" + str(i))

def main(a):
    if not a:
        print(__doc__); return 2
    cmd, args = a[0], a[1:]
    if cmd == "list":
        for i in free()[: int(args[0]) if args else None]:
            print(show(i))
    elif cmd == "next":
        rev = "--from-end" in args
        args = [x for x in args if x != "--from-end"]
        who, n = args[0], int(args[1]) if len(args) > 1 else 1
        got = []
        for i in (free()[::-1] if rev else free()):
            if claim(i, who):
                got.append(i)
                print(show(i))
            if len(got) == n:
                break
    elif cmd == "claim":
        if int(args[1]) in registered() or not claim(int(args[1]), args[0]):
            print(f"{args[1]} is taken or already ported", file=sys.stderr); return 1
    elif cmd == "release":
        d = os.path.join(CLAIMS, args[0])
        for f in os.listdir(d):
            os.remove(os.path.join(d, f))
        os.rmdir(d)
    elif cmd == "done":
        os.makedirs(os.path.join(CLAIMS, args[0]), exist_ok=True)
        open(os.path.join(CLAIMS, args[0], "done"), "w").write("")
    elif cmd == "audit-next":
        who, n = args[0], int(args[1]) if len(args) > 1 else 1
        rows, got = audit_open(), 0
        for i in sorted(rows):
            if got < n and audit_state(i) is None and claim("audit-" + str(i), who):
                got += 1
                print(f"{i}\t{'; '.join(rows[i])}")
    elif cmd == "audit-done":
        os.makedirs(os.path.join(CLAIMS, "audit-" + args[0]), exist_ok=True)
        open(os.path.join(CLAIMS, "audit-" + args[0], "done"), "w").write("")
    elif cmd == "audit-status":
        rows = audit_open()
        held = {i: audit_state(i) for i in rows if audit_state(i) not in (None, "done")}
        print(f"open audit rows for {len(rows)} handlers, claimed {len(held)}, free {len([i for i in rows if audit_state(i) is None])}")
        for i, w in sorted(held.items()):
            print(f"  {i}\t{w}")
    elif cmd == "status":
        h, reg = handlers(), registered()
        held = {i: state(i) for i in h if state(i) not in (None, "done")}
        print(f"handlers {len(h)}, registered in Go {len(reg)}, claimed {len(held)}, free {len(free())}")
        for i, w in sorted(held.items()):
            print(f"  {i}\t{w}\t{h[i][1]}")
    return 0

sys.exit(main(sys.argv[1:]))
