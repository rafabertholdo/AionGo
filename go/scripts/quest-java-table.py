#!/usr/bin/env python3
"""Print a normalized event table for one Aion 1.9 Java quest handler.

    scripts/quest-java-table.py <questId> [--json] [--raw-guards]

Heuristic parser (brace/statement tracking, not a Java parser): every row carries the original
Java line number (L<n>) so the reading can be verified. It prints
  * quest_data.xml facts (name, race, min level, prerequisites, rewards, items, drops),
  * the XML template declaration (quest_script_data) when the quest is a template quest,
  * register(): npc/item/zone ids per event,
  * per event method (onDialogEvent, onKillEvent, onLvlUpEvent, ...): rows `L<line> [guards] -> action`,
    where guards are the enclosing status/var/npc/dialog conditions and the action is a window, movie,
    var change, status change, item grant/removal, service call, `return true|false` or `break`.
`--json` prints the facts other scripts need: min level, race, prerequisites, start/talk npcs, level-up
start, quest item ids. `--shell` prints the same as shell assignments plus the min experience and the start
npc's spawn position (quest-debug.sh prep evals it).
Reading aids: `return true` = the handler answered; `return false` = the framework echoes the page
(see QUEST_HANDLER_AGENT.md). `break` under a switch continues AFTER the switch (often into the
next npc's branch or the final return). `FALLTHROUGH` marks a case that has no break/return.
"""
import glob
import json
from pathlib import Path
import os
import re
import sys
import xml.etree.ElementTree as ET

JAVA = os.environ.get("AION_JAVA", str(Path(__file__).resolve().parents[2] / "java/AL-Game"))
HANDLERS = JAVA + "/data/scripts/system/handlers/quest"
STATIC = JAVA + "/data/static_data"


def find_handler(qid):
    for path in sorted(glob.glob(f"{HANDLERS}/*/_{qid}*.java")):
        if re.match(rf"_{qid}(?!\d)", os.path.basename(path)):
            return path
    return None


# ---------------------------------------------------------------- tokenizer

def tokenize(src):
    """Yield (kind, text, line): kind is 'open' (header before {), 'close', 'stmt' or 'label'.
    Comments and strings are blanked; trailing // comments are returned in a line->text map."""
    comments = {}
    out, buf, paren, line, buf_line, buf_empty = [], [], 0, 1, 1, True
    i, n = 0, len(src)

    def flush(kind):
        nonlocal buf, buf_empty
        text = re.sub(r"\s+", " ", "".join(buf)).strip()
        if text:
            out.append((kind, text, buf_line))
        buf, buf_empty = [], True

    while i < n:
        ch = src[i]
        if src.startswith("//", i):
            j = src.find("\n", i)
            j = n if j < 0 else j
            comments[line] = src[i + 2:j].strip()
            i = j
            continue
        if src.startswith("/*", i):
            j = src.find("*/", i + 2)
            j = n if j < 0 else j + 2
            line += src.count("\n", i, j)
            i = j
            continue
        if buf_empty and not ch.isspace():
            buf_line, buf_empty = line, False
        if ch in "\"'":
            j = i + 1
            while j < n and src[j] != ch:
                j += 2 if src[j] == "\\" else 1
            buf.append(src[i:j + 1] if ch == '"' else "'c'")
            i = j + 1
            continue
        if ch == "\n":
            line += 1
            buf.append(" ")
        elif ch == "(":
            paren += 1
            buf.append(ch)
        elif ch == ")":
            paren -= 1
            buf.append(ch)
        elif paren <= 0 and ch == "{":
            flush("open")
        elif paren <= 0 and ch == "}":
            flush("stmt")
            out.append(("close", "}", line))
        elif paren <= 0 and ch == ";":
            buf.append(";")
            flush("stmt")
        elif paren <= 0 and ch == ":" and re.match(r"\s*(case\b|default\s*$)", "".join(buf)) and "?" not in "".join(buf):
            flush("label")
        else:
            buf.append(ch)
        i += 1
    return out, comments


# ---------------------------------------------------------------- normalizing

ALIASES = {}


def norm_cond(text):
    t = text
    for name, value in ALIASES.items():
        t = re.sub(rf"\b{name}\b", value, t)
    subs = [
        (r"qs\.getStatus\(\)\s*(==|!=)\s*QuestStatus\.(\w+)", lambda m: f"status{m[1]}{m[2]}"),
        (r"env\.getDialogId\(\)\s*(==|!=)\s*(-?\d+)", lambda m: f"dialog{m[1]}{m[2]}"),
        (r"targetId\s*(==|!=)\s*(\d+)", lambda m: f"npc{m[1]}{m[2]}"),
        (r"qs\.getQuestVarById\((\d+)\)", lambda m: f"var{m[1]}" if m[1] != "0" else "var"),
        (r"\bvar\s*(==|!=|>=|<=|>|<)\s*(-?\d+)", lambda m: f"var{m[1]}{m[2]}"),
        (r"player\.getInventory\(\)\.getItemCountByItemId\((\d+)\)", lambda m: f"count({m[1]})"),
        (r"\(\(Npc\) env\.getVisibleObject\(\)\)\.getNpcId\(\)", lambda m: "npcId"),
        (r"env\.getVisibleObject\(\) instanceof Npc", lambda m: "isNpc"),
        (r"QuestStatus\.", lambda m: ""),
    ]
    for pat, rep in subs:
        t = re.sub(pat, rep, t)
    return t


def matching_paren(text, start):
    depth = 0
    for i in range(start, len(text)):
        depth += text[i] == "("
        depth -= text[i] == ")"
        if depth == 0:
            return i
    return len(text) - 1


HEADER = re.compile(r"^(else\s+if|if|for|while|switch|synchronized)\s*\(")


def split_header(text):
    """('if'|'else if'|..., cond, rest) for a statement that starts with a control header, else None.
    'else' alone returns ('else', '', rest)."""
    m = HEADER.match(text)
    if m:
        end = matching_paren(text, m.end() - 1)
        return re.sub(r"\s+", " ", m[1]), text[m.end():end], text[end + 1:].strip()
    m = re.match(r"^else\b\s*(.*)$", text)
    if m:
        return "else", "", m[1].strip()
    return None


ACTIONS = [
    (r"sendQuestDialog\(\s*\w+\s*,\s*[^,]+,\s*(\d+)\s*\)", lambda m: f"window {m[1]} (sendQuestDialog)"),
    (r"new SM_DIALOG_WINDOW\(\s*[^,)]+,\s*(\d+)\s*(?:,\s*(\w+)\s*)?\)", lambda m: f"window {m[1]} (raw SM_DIALOG_WINDOW)"),
    (r"new SM_PLAY_MOVIE\(\s*\d+\s*,\s*(\d+)\s*\)", lambda m: f"MOVIE {m[1]}"),
    (r"defaultQuestStartDialog\(", lambda m: "defaultQuestStartDialog (1007->window 4, 1003->1004, 1002->start+window 1003)"),
    (r"defaultQuestEndDialog\(", lambda m: "defaultQuestEndDialog (8-17 -> questFinish+window 10; 1009/-1 -> window 5 in REWARD)"),
    (r"setQuestVarById\(\s*(\d+)\s*,\s*([^;{}]+?)\)\s*(?:;|$|\))", lambda m: f"setQuestVar[{m[1]}] = {norm_cond(m[2])}"),
    (r"setQuestVar\(\s*([^;{}]+?)\)\s*(?:;|$)", lambda m: f"setQuestVar = {norm_cond(m[1])}"),
    (r"\.setStatus\(\s*QuestStatus\.(\w+)\s*\)", lambda m: f"setStatus {m[1]}"),
    (r"updateQuestStatus\(", lambda m: "updateQuestStatus (SM_QUEST_ACCEPTED)"),
    (r"QuestService\.startQuest\(([^;]*)\)", lambda m: f"QuestService.startQuest({m[1]})"),
    (r"QuestService\.questFinish\(", lambda m: "QuestService.questFinish"),
    (r"QuestService\.collectItemCheck\(", lambda m: "collectItemCheck (removes items when complete)"),
    (r"ItemService\.addItems?\(([^;]*)\)", lambda m: f"GIVE {norm_cond(m[1])}"),
    (r"ItemService\.removeItemFromInventoryByItemId\(\s*\w+\s*,\s*([^)]+)\)", lambda m: f"REMOVE ALL of item {m[1]}"),
    (r"ItemService\.decreaseItemCountByItemId\(\s*\w+\s*,\s*([^)]+)\)", lambda m: f"REMOVE item {m[1].replace(',', ' x')}"),
    (r"\b(QuestService|ItemService|TeleportService|SkillLearnService|WorldMapInstanceFactory|InstanceService|ThreadPoolManager|WorldMapInstance)\.(\w+)\(", lambda m: f"call {m[1]}.{m[2]}"),
    (r"PacketSendUtility\.(\w+)\(\s*\w+\s*,\s*(?:new )?(\w+)", lambda m: f"send {m[2]}"),
    (r"\bspawn\w*\(", lambda m: "spawn"),
]


def actions_of(text):
    out = []
    body = text.rstrip(";").strip()
    for pat, fmt in ACTIONS:
        for m in re.finditer(pat, body):
            s = fmt(m)
            if s not in out and not (s.startswith("send SM_DIALOG_WINDOW") or s.startswith("send SM_PLAY_MOVIE") or s.startswith("call QuestService.startQuest")):
                out.append(s)
    return out


# ---------------------------------------------------------------- method walker

class Frame:
    def __init__(self, kind, cond="", braceless=False):
        self.kind, self.cond, self.braceless = kind, cond, braceless
        self.labels = []      # switch: current case labels
        self.label_open = False
        self.terminal = False  # switch: the last statement of the current case was a break/return
        self.label_body = False  # switch: the current case has at least one statement


def switch_name(cond):
    if "getDialogId" in cond:
        return "dialog"
    if cond.strip() in ("targetId", "npcId"):
        return "npc"
    return norm_cond(cond)


def guards(stack, raw):
    out = []
    for fr in stack[1:]:
        if fr.kind == "switch":
            if fr.labels:
                out.append(f"{switch_name(fr.cond)}={'|'.join(fr.labels)}")
        elif fr.kind == "else":
            out.append("else")
        elif fr.kind in ("if", "else if"):
            c = fr.cond if raw else norm_cond(fr.cond)
            out.append(("else " if fr.kind == "else if" else "") + c)
        elif fr.kind == "block":
            out.append(fr.cond)
        else:
            out.append(f"{fr.kind}({fr.cond if raw else norm_cond(fr.cond)})")
    return out


def walk(tokens, raw=False):
    """Return {method: [(line, guards, action)]}, in file order; register() rows are raw statements."""
    methods, current, stack, pending = {}, None, [], None
    METHOD = re.compile(r"^(?:@\w+\s+)*public\s+\w+\s+(on\w+|register)\s*\(")

    def emit(line, action):
        methods[current].append((line, guards(stack, raw), action))

    def pop_braceless():
        while len(stack) > 1 and stack[-1].braceless:
            stack.pop()

    def handle_stmt(line, text):
        alias = re.match(r"^(?:final )?(?:long|int) (\w+) = .*getItemCountByItemId\((\d+)\)", text)
        if alias:
            ALIASES[alias[1]] = f"count({alias[2]})"
        hdr = split_header(text)
        if hdr and hdr[0] != "switch" and hdr[2]:      # braceless if/else/for/while
            kind, cond, rest = hdr
            stack.append(Frame(kind, cond, braceless=True))
            stack[-1].terminal = False
            handle_stmt(line, rest)
            return
        m = re.match(r"^(return|throw)\b\s*(.*?);?$", text)
        if m:
            if m[2] in ("true", "false", ""):
                emit(line, f"{m[1]} {m[2]}".strip() + ("   <- framework echoes the page" if m[2] == "false" else "   <- handler answered" if m[2] == "true" else ""))
            else:
                acts = actions_of(m[2])
                emit(line, (" ; ".join(acts) if acts else m[2]) + "   [return of this call]")
            end_terminal(True)
        elif re.match(r"^break\s*;", text):
            in_switch = any(f.kind == "switch" for f in stack)
            emit(line, "break -> leaves the switch, continues after it" if in_switch else "break")
            end_terminal(True)
        elif text.endswith(";"):
            acts = actions_of(text)
            if acts:
                for a in acts:
                    emit(line, a)
            end_terminal(False)
        pop_braceless()

    def end_terminal(value):
        # terminal only counts for the frame that directly holds the case body
        conditional = False
        for f in reversed(stack):
            if f.kind == "switch":
                f.terminal = value and not conditional
                break
            if not f.braceless:
                break
            conditional = True

    for kind, text, line in tokens:
        if current is None:
            m = METHOD.match(text) if kind == "open" else None
            if m:
                current = m[1]
                methods.setdefault(current, [])
                stack = [Frame("method")]
            continue
        if kind == "open":
            hdr = split_header(text)
            if hdr:
                k, cond, rest = hdr
                if rest and k != "switch":       # `if (x) for (...) {` : header + braceless parent
                    stack.append(Frame(k, cond, braceless=True))
                    inner = split_header(rest)
                    stack.append(Frame(inner[0] if inner else "block", inner[1] if inner else rest))
                else:
                    stack.append(Frame(k, cond))
            elif re.match(r"^(try|finally|do)$|^catch\b", text):
                stack.append(Frame("block", text))
            elif "new " in text or "run()" in text:
                stack.append(Frame("block", "async " + text[-40:]))
                emit(line, "anonymous class / timer body starts here")
            else:
                stack.append(Frame("block", text[-40:]))
            continue
        if kind == "label":
            sw = next((f for f in reversed(stack) if f.kind == "switch"), None)
            if sw is None:
                continue
            label = "default" if text.startswith("default") else re.sub(r"^case\s+", "", text)
            if sw.label_open and not sw.terminal:
                if sw.terminal is False and sw.labels and sw.label_body:
                    emit(line, f"FALLTHROUGH: the previous case ({'|'.join(sw.labels)}) has no break/return, it continues into {label}")
                    sw.labels = [label]
                else:
                    sw.labels = sw.labels + [label]       # stacked labels: case 1: case 2:
            else:
                sw.labels = [label]
            sw.label_open, sw.terminal, sw.label_body = True, False, False
            continue
        if kind == "close":
            if len(stack) == 1:
                # end of method: a boolean method must return explicitly, nothing implicit to report
                current = None
                stack = []
                continue
            stack.pop()
            if stack:
                stack[-1].terminal = False
            pop_braceless()
            continue
        # plain statement
        for f in stack:
            if f.kind == "switch":
                f.label_body = True
        handle_stmt(line, text)
    return methods


# ---------------------------------------------------------------- register + xml facts

EVENT_NAMES = {"addOnTalkEvent": "talk", "addOnKillEvent": "kill", "addOnQuestStart": "questStart(npc offers it)",
               "addQuestLvlUp": "levelUp", "addOnAttackEvent": "attack", "addOnDie": "die", "addOnEnterWorld": "enterWorld",
               "addOnQuestFinish": "questFinish"}


def register_rows(tokens, comments):
    rows = []
    for kind, text, line in tokens:
        if kind != "stmt" or not text.startswith("qe."):
            continue
        ids = re.findall(r"\((\w[\w.]*)\)", text)
        m = re.match(r"qe\.(?:setNpcQuestData\((\d+)\)\.(\w+)|setQuestItemIds\((\d+)\)\.(\w+)|setQuestEnterZone\(([\w.]+)\)\.(\w+)|setQuestMovieEndIds\((\d+)\)\.(\w+)|(\w+))", text)
        if not m:
            continue
        if m[1]:
            rows.append((line, m[2], m[1], comments.get(line, "")))
        elif m[3]:
            rows.append((line, "itemUse", m[3], comments.get(line, "")))
        elif m[5]:
            rows.append((line, "enterZone", m[5], comments.get(line, "")))
        elif m[7]:
            rows.append((line, "movieEnd", m[7], comments.get(line, "")))
        else:
            rows.append((line, m[9], ",".join(ids), comments.get(line, "")))
    return rows


_ROOT = []


def quest_data(qid):
    facts = {"id": qid}
    try:
        if not _ROOT:
            _ROOT.append(ET.parse(f"{STATIC}/quest_data/quest_data.xml").getroot())
        root = _ROOT[0]
    except (OSError, ET.ParseError):
        return facts
    for q in root.iter("quest"):
        if q.get("id") == str(qid):
            facts.update(name=q.get("name"), race=q.get("race_permitted", "ALL"), minlevel=int(q.get("minlevel_permitted", "0") or 0),
                         attrs={k: v for k, v in q.attrib.items() if k not in ("id", "name", "nameId")},
                         children=[(c.tag, dict(c.attrib), [(g.tag, dict(g.attrib)) for g in c]) for c in q])
            facts["prerequisites"] = [int(x) for c in q if c.tag == "finished_quest_conds" for x in (c.text or "").split() if x.isdigit()]
            facts["children_text"] = {c.tag: (c.text or "").strip() for c in q if (c.text or "").strip()}
            return facts
    return facts


def min_exp(level):
    """Total experience a level starts at (player_experience_table.xml, index level-1)."""
    try:
        values = [int(e.text) for e in ET.parse(f"{STATIC}/player_experience_table.xml").getroot().iter("exp")]
    except (OSError, ET.ParseError):
        return 0
    return values[level - 1] if 1 <= level <= len(values) else 0


def npc_spawn(npc):
    """(map, x, y, z, heading) of the first spawn of an npc template in spawns/Npcs/*.xml, or None."""
    pat = re.compile(rf'<spawn\s+map="(\d+)"\s+npcid="{npc}"[^>]*>\s*<object\s+x="([-\d.]+)"\s+y="([-\d.]+)"\s+z="([-\d.]+)"(?:\s+h="(\d+)")?')
    for path in sorted(glob.glob(f"{STATIC}/spawns/Npcs/*.xml")):
        with open(path, encoding="utf-8", errors="replace") as f:
            m = pat.search(f.read())
        if m:
            return m[1], m[2], m[3], m[4], m[5] or "0"
    return None


def xml_template(qid):
    for path in sorted(glob.glob(f"{STATIC}/quest_script_data/*.xml")):
        try:
            root = ET.parse(path).getroot()
        except ET.ParseError:
            continue
        for el in root:
            if el.get("id") == str(qid):
                return os.path.basename(path), el
    return None, None


def describe_xml(el):
    head = f"<{el.tag} " + " ".join(f'{k}="{v}"' for k, v in el.attrib.items()) + ">"
    kids = ["    " + f"<{c.tag} " + " ".join(f'{k}="{v}"' for k, v in c.attrib.items()) + "/>" for c in el]
    return "\n".join([head] + kids)


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    flags = {a for a in sys.argv[1:] if a.startswith("--")}
    if len(args) != 1 or not args[0].isdigit():
        sys.exit(__doc__)
    qid = int(args[0])
    path = find_handler(qid)
    facts = quest_data(qid)
    xml_file, xml_el = xml_template(qid)
    rows, methods, tokens, comments = [], {}, [], {}
    if path:
        with open(path, encoding="utf-8", errors="replace") as f:
            tokens, comments = tokenize(f.read())
        rows = register_rows(tokens, comments)
        methods = walk(tokens, raw="--raw-guards" in flags)
    lvlup = any(r[1] == "addQuestLvlUp" for r in rows)
    start_npcs = [int(r[2]) for r in rows if r[1] == "addOnQuestStart"]
    if xml_el is not None and xml_el.get("start_npc_id"):
        start_npcs.append(int(xml_el.get("start_npc_id")))
    quest_items = {int(r[2]) for r in rows if r[1] == "itemUse"}
    for tag, attrs, kids in facts.get("children", []):
        if tag == "quest_drop" and "item_id" in attrs:
            quest_items.add(int(attrs["item_id"]))
        quest_items |= {int(a["item_id"]) for g, a in kids if "item_id" in a and tag in ("collect_items", "quest_work_items")}
    quest_items = sorted(quest_items)
    talk_npcs = [int(r[2]) for r in rows if r[1] == "addOnTalkEvent"]
    if xml_el is not None and xml_el.get("start_npc_id") and not talk_npcs:
        talk_npcs = [int(xml_el.get("start_npc_id"))]
    chain, todo = [], list(facts.get("prerequisites", []))
    while todo:  # prerequisites of prerequisites, so a chain quest can be started from a clean character
        q = todo.pop(0)
        if q not in chain:
            chain.append(q)
            todo += quest_data(q).get("prerequisites", [])
    if "--shell" in flags:  # numbers only, safe to eval: quest-debug.sh prep reads this
        anchor = (start_npcs or talk_npcs or [0])[0]
        spawn = npc_spawn(anchor) if anchor else None
        values = {"q_name": None, "q_minlevel": facts.get("minlevel", 0), "q_minexp": min_exp(facts.get("minlevel", 0) or 1),
                  "q_levelup": int(lvlup), "q_prereqs": " ".join(map(str, sorted(chain))),
                  "q_items": " ".join(map(str, quest_items)), "q_npc": anchor, "q_race": None,
                  "q_world": spawn[0] if spawn else 0, "q_x": spawn[1] if spawn else 0, "q_y": spawn[2] if spawn else 0,
                  "q_z": spawn[3] if spawn else 0, "q_h": spawn[4] if spawn else 0, "q_found": int(bool(facts.get("name")))}
        for key, value in values.items():
            if value is not None:
                print(f'{key}="{value}"')
        print(f'q_race={facts.get("race", "ALL")}')
        return
    if "--json" in flags:
        print(json.dumps({"id": qid, "handler": path, "template": xml_el.tag if xml_el is not None else None,
                          "name": facts.get("name"), "race": facts.get("race"), "minlevel": facts.get("minlevel", 0),
                          "prerequisites": facts.get("prerequisites", []), "levelup_start": lvlup,
                          "start_npcs": start_npcs, "talk_npcs": talk_npcs, "quest_items": quest_items,
                          "reward_items": sorted({int(a["item_id"]) for _, _, kids in facts.get("children", []) for g, a in kids if "reward" in g and "item_id" in a}),
                          "events": sorted({r[1] for r in rows})}))
        return
    print(f"quest {qid}: {facts.get('name', '(not in quest_data.xml)')}")
    print(f"  handler : {path or '(no Java handler script)'}")
    if facts.get("attrs"):
        print(f"  quest_data: race={facts['race']} minlevel={facts['minlevel']} prerequisites={facts.get('prerequisites') or '-'} "
              + " ".join(f"{k}={v}" for k, v in facts["attrs"].items() if k not in ("race_permitted", "minlevel_permitted")))
        for tag, attrs, kids in facts["children"]:
            print(f"    <{tag} {' '.join(f'{k}={v}' for k, v in attrs.items())}>{facts.get('children_text', {}).get(tag, '')} " + " ".join(f"{g}({' '.join(f'{k}={v}' for k, v in a.items())})" for g, a in kids))
    print(f"  starts  : {'ON LEVEL-UP (needs a START/0 row when reset: RESET_TO=START)' if lvlup else 'by npc ' + ','.join(map(str, start_npcs)) if start_npcs else 'unknown'}")
    if xml_el is not None:
        print(f"\nXML template ({xml_file}) - no per-quest Java code, the {xml_el.tag} handler class drives it:")
        print("  " + describe_xml(xml_el).replace("\n", "\n  "))
    if rows:
        print("\nregister():")
        for line, event, ids, comment in rows:
            print(f"  L{line:<4} {EVENT_NAMES.get(event, event):<26} {ids if ids != 'questId' else ''}{'   // ' + comment if comment else ''}")
    for name, items in methods.items():
        if name == "register" or not items:
            continue
        print(f"\n{name}:")
        last = None
        for line, gs, action in items:
            g = " > ".join(gs) or "(always)"
            print(f"  L{line:<4} {g if g != last else '  ^ same':<58} -> {action}")
            last = g
    if not path and xml_el is None:
        print("\nNo handler and no XML template: the quest is data-only (quest_data.xml).")


if __name__ == "__main__":
    main()
