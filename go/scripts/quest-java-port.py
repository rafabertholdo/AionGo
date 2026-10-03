#!/usr/bin/env python3
"""Translate Java quest handlers' onDialogEvent into Go, branch for branch.

The Java 1.9 handlers use a small API (quest state and variables, dialog windows, the default start/end dialogs,
quest items, movies). This turns each listed handler's onDialogEvent into

    func (c *conn) javaDialog<ID>(o *object, script *data.QuestScript, d int32) bool

returning Java's boolean, and writes them with the javaDialogs registry to game/quest_java_dialogs.go. A handler
using anything outside that API is reported and left out (port it by hand). TestQuestJavaParity checks the result.

    scripts/quest-java-port.py 1097 3319 ...     translate these (adds to / replaces in the generated file)
    scripts/quest-java-port.py --check 1097      print the Go for one handler, or why it cannot be translated
"""
import glob
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
JAVA = os.path.join(ROOT, '..', 'java', 'AL-Game', 'data', 'scripts', 'system', 'handlers', 'quest')
OUT = os.path.join(ROOT, 'game', 'quest_java_dialogs.go')


class Unsupported(Exception):
    pass


# ---- tokens

TOKEN = re.compile(r'''
    (?P<ws>\s+|//[^\n]*|/\*.*?\*/)
  | (?P<num>0[xX][0-9a-fA-F]+[lL]?|\d+\.\d*[fF]?|\d+[fFlL]?)
  | (?P<str>"(?:\\.|[^"\\])*")
  | (?P<id>[A-Za-z_][A-Za-z0-9_]*)
  | (?P<op>==|!=|<=|>=|&&|\|\||\+\+|--|\+=|-=|[-+*/%<>=!?:;,.(){}\[\]&|@])
''', re.S | re.X)


def tokenize(src):
    out, pos = [], 0
    while pos < len(src):
        m = TOKEN.match(src, pos)
        if not m:
            raise Unsupported('cannot tokenize at %r' % src[pos:pos + 20])
        pos = m.end()
        kind = m.lastgroup
        if kind != 'ws':
            text = m.group()
            if kind == 'num':
                text = text.rstrip('lLfF')
            out.append((kind, text))
    out.append(('eof', ''))
    return out


# ---- parser (a Java subset) into tuples

class Parser:
    def __init__(self, toks):
        self.t, self.i = toks, 0

    def peek(self, k=0):
        return self.t[self.i + k][1]

    def kind(self, k=0):
        return self.t[self.i + k][0]

    def eat(self, text=None):
        tok = self.t[self.i]
        if text is not None and tok[1] != text:
            raise Unsupported('expected %r, got %r' % (text, tok[1]))
        self.i += 1
        return tok[1]

    def block(self):
        self.eat('{')
        out = []
        while self.peek() != '}':
            out.append(self.stmt())
        self.eat('}')
        return ('block', out)

    def stmt(self):
        p = self.peek()
        if p == '{':
            return self.block()
        if p == ';':
            self.eat()
            return ('block', [])
        if p == 'if':
            self.eat()
            self.eat('(')
            cond = self.expr()
            self.eat(')')
            then = self.stmt()
            other = None
            if self.peek() == 'else':
                self.eat()
                other = self.stmt()
            return ('if', cond, then, other)
        if p == 'switch':
            self.eat()
            self.eat('(')
            subject = self.expr()
            self.eat(')')
            self.eat('{')
            cases = []  # (labels or None for default, [stmts])
            while self.peek() != '}':
                if self.peek() == 'case':
                    self.eat()
                    label = self.expr()
                    self.eat(':')
                    cases.append(([label], []))
                elif self.peek() == 'default':
                    self.eat()
                    self.eat(':')
                    cases.append((None, []))
                else:
                    if not cases:
                        raise Unsupported('statement before case')
                    cases[-1][1].append(self.stmt())
            self.eat('}')
            return ('switch', subject, cases)
        if p == 'return':
            self.eat()
            value = None if self.peek() == ';' else self.expr()
            self.eat(';')
            return ('return', value)
        if p == 'break':
            self.eat()
            self.eat(';')
            return ('break',)
        if p == 'continue':
            self.eat()
            self.eat(';')
            return ('continue',)
        if p == 'for':
            self.eat()
            self.eat('(')
            if self.peek() == 'final':
                self.eat()
            elemtype = self.eat()
            name = self.eat()
            self.eat(':')
            seq = self.expr()
            self.eat(')')
            return ('foreach', name, seq, self.stmt(), elemtype)
        if p in ('while', 'do', 'try', 'throw', 'synchronized'):
            raise Unsupported(p)
        if p in ('++', '--'):
            op = self.eat()
            target = self.expr()
            self.eat(';')
            return ('assign', target, '+=' if op == '++' else '-=', ('num', '1'))
        if p == 'int' and self.peek(1) == '[' and self.peek(2) == ']':
            self.i += 3
            name = self.eat()
            self.eat('=')
            self.eat('{')
            items = []
            while self.peek() != '}':
                items.append(self.expr())
                if self.peek() == ',':
                    self.eat()
            self.eat('}')
            self.eat(';')
            return ('array', name, items)
        # local declaration: [final] Type name [= expr];
        j = self.i
        if self.peek() == 'final':
            j += 1
        if (self.t[j][0] == 'id' and self.t[j + 1][0] == 'id' and self.t[j + 2][1] in ('=', ';')
                and self.t[j][1] not in ('return',)):
            self.i = j
            jtype = self.eat()
            name = self.eat()
            init = None
            if self.peek() == '=':
                self.eat()
                init = self.expr()
            self.eat(';')
            return ('decl', jtype, name, init)
        e = self.expr()
        if self.peek() in ('=', '+=', '-='):
            op = self.eat()
            value = self.expr()
            self.eat(';')
            return ('assign', e, op, value)
        if self.peek() in ('++', '--'):
            op = self.eat()
            self.eat(';')
            return ('assign', e, '+=' if op == '++' else '-=', ('num', '1'))
        self.eat(';')
        return ('expr', e)

    PREC = [('||',), ('&&',), ('==', '!='), ('<', '>', '<=', '>='), ('+', '-'), ('*', '/', '%')]

    def expr(self, level=0):
        if level == len(self.PREC):
            return self.unary()
        left = self.expr(level + 1)
        while self.peek() in self.PREC[level]:
            op = self.eat()
            right = self.expr(level + 1)
            left = ('bin', op, left, right)
        if self.peek() == 'instanceof':
            self.eat()
            return ('instanceof', left, self.eat())
        if level == 0 and self.peek() == '?':
            raise Unsupported('?:')
        return left

    def unary(self):
        if self.peek() in ('++', '--'):
            op = self.eat()
            return ('preinc', op, self.unary())
        if self.peek() == '!':
            self.eat()
            return ('not', self.unary())
        if self.peek() == '-':
            self.eat()
            return ('neg', self.unary())
        return self.postfix(self.primary())

    def primary(self):
        k, p = self.kind(), self.peek()
        if p == '(' and self.peek(1) in ('byte', 'int', 'long', 'float', 'short', 'double') and self.peek(2) == ')':
            self.i += 3
            return ('cast', self.t[self.i - 2][1], self.unary())
        if p == '(':
            # cast "(Npc) x" or parenthesised expression
            if self.kind(1) == 'id' and self.peek(2) == ')' and self.peek(1)[0].isupper() and self.kind(3) in ('id', 'op') and self.peek(3) not in (
                    ')', ';', '==', '!=', '&&', '||', '+', '-', '*', '/', '<', '>', '<=', '>=', ','):
                self.eat('(')
                self.eat()
                self.eat(')')
                return self.unary()
            self.eat('(')
            e = self.expr()
            self.eat(')')
            return ('paren', e)
        if k == 'num':
            return ('num', self.eat())
        if k == 'str':
            return ('str', self.eat())
        if p == 'new':
            self.eat()
            name = self.eat()
            while self.peek() == '.':
                self.eat()
                name += '.' + self.eat()
            if self.peek() == '<':
                raise Unsupported('generic new')
            args = self.args()
            if self.peek() == '{':
                if name != 'Runnable':
                    raise Unsupported('anonymous ' + name)
                self.eat('{')
                fields = []
                while self.peek() not in ('@', 'public'):
                    fields.append(self.stmt())  # field initializers run when the task is created
                if self.peek() == '@':
                    self.eat()
                    self.eat()
                for word in ('public', 'void', 'run', '(', ')'):
                    self.eat(word)
                body = self.block()
                self.eat('}')
                return ('runnable', body, fields)
            return ('new', name, args)
        if k == 'id':
            return ('name', self.eat())
        raise Unsupported('unexpected %r' % p)

    def args(self):
        self.eat('(')
        out = []
        while self.peek() != ')':
            out.append(self.expr())
            if self.peek() == ',':
                self.eat()
        self.eat(')')
        return out

    def postfix(self, e):
        while True:
            if self.peek() == '.':
                self.eat()
                name = self.eat()
                if self.peek() == '(':
                    e = ('call', e, name, self.args())
                else:
                    e = ('field', e, name)
            elif self.peek() == '(' and e[0] == 'name':
                e = ('call', None, e[1], self.args())
            else:
                return e


# ---- Go emission

def dotted(e):
    """The expression as a dotted string when it is a plain chain of names, calls without arguments and fields."""
    if e[0] == 'name':
        return e[1]
    if e[0] == 'field':
        return dotted(e[1]) + '.' + e[2]
    if e[0] == 'call' and e[1] is not None:
        return dotted(e[1]) + '.' + e[2] + '()'
    if e[0] == 'call':
        return e[2] + '()'
    if e[0] == 'paren':
        return dotted(e[1])
    return '?'


class Emitter:
    def __init__(self, qid, src=''):
        self.qid = qid
        self.src = src
        self.helpers = {}  # name -> Go source
        self.locals = {}  # java name -> kind: quest, obj, int, long, bool
        self.pre = []
        self.closure = 0

    # expressions: (go, type) with type in quest|obj|int|long|bool|str|status|any
    def x(self, e):
        go, _ = self.ex(e)
        return go

    def ex(self, e):
        t = e[0]
        if t == 'true':
            return 'true', 'bool'
        if t == 'instanceof':
            if e[1][0] == 'name' and self.locals.get(e[1][1]) == 'obj' and e[2] == 'Npc':
                return '%s.npc != nil' % goname(e[1][1]), 'bool'
            return 'true', 'bool'  # env.getVisibleObject() is an npc, a target a creature
        if t == 'num':
            return e[1], 'num'
        if t == 'str':
            return e[1], 'str'
        if t == 'paren':
            go, ty = self.ex(e[1])
            return '(' + go + ')', ty
        if t == 'cast':
            go, ty = self.ex(e[2])
            gotype = {'byte': 'byte', 'int': 'int32', 'long': 'int64', 'float': 'float32', 'short': 'int16', 'double': 'float64'}[e[1]]
            return '%s(%s)' % (gotype, go), ('int' if e[1] in ('int', 'short') else 'long' if e[1] == 'long' else 'num')
        if t == 'preinc':
            target = e[2]
            if target[0] != 'name' or target[1] not in self.locals:
                raise Unsupported('++ of ' + dotted(target))
            self.pre.append('%s %s 1' % (goname(target[1]), '+=' if e[1] == '++' else '-='))
            return goname(target[1]), self.locals[target[1]]
        if t == 'not':
            return '!' + self.paren(e[1]), 'bool'
        if t == 'neg':
            return '-' + self.paren(e[1]), 'num'
        if t == 'bin':
            op, a, b = e[1], e[2], e[3]
            ga, ta = self.ex(a)
            gb, tb = self.ex(b)
            if op in ('==', '!=') and 'null' in (ga, gb):
                return '%s %s %s' % (ga, op, gb), 'bool'
            if ta == 'long' and tb == 'int':
                gb = 'int64(%s)' % gb
            elif tb == 'long' and ta == 'int':
                ga = 'int64(%s)' % ga
            ty = 'bool' if op in ('==', '!=', '<', '>', '<=', '>=', '&&', '||') else (ta if ta != 'num' else tb)
            if op in ('||', '&&') and ga == gb:
                return ga, ty  # "var == 0 || var == 0" in the Java
            return '%s %s %s' % (self.wrap(a, ga, op), op, self.wrap(b, gb, op)), ty
        if t == 'name':
            n = e[1]
            if n == 'null':
                return 'nil', 'null'
            if n in ('true', 'false'):
                return n, 'bool'
            if n == 'questId':
                return 'script.ID', 'int'
            if n == 'player':
                return 'p', 'player'
            if n in self.locals:
                kind = self.locals[n]
                return goname(n), kind
            raise Unsupported('name ' + n)
        s = dotted(e)
        fixed = {
            'env.getDialogId()': ('d', 'int'),
            'env.getQuestId()': ('script.ID', 'int'),
            'env.getVisibleObject()': ('o', 'obj'),
            'env.getVisibleObject().getObjectId()': ('o.id', 'int'),
            'env.getVisibleObject().getNpcId()': ('o.npc.ID', 'int'),
            'env.getPlayer()': ('p', 'player'),
            'player.getObjectId()': ('p.ID', 'int'),
            'player.getCommonData().getLevel()': ('int32(p.level)', 'int'),
            'player.getLevel()': ('int32(p.level)', 'int'),
            'player.getCommonData().getRace()': ('p.Race', 'enum'),
            'player.getCommonData().getPlayerClass()': ('p.Class', 'enum'),
            'player.getCommonData().getGender()': ('p.Gender', 'enum'),
            'player.getInstanceId()': ('p.instance', 'int'),
            'player.getWorldId()': ('p.WorldID', 'int'),
            'player.getTarget()': ('c.jTarget()', 'obj'),
        }
        if s in fixed:
            return fixed[s]
        m = re.fullmatch(r'(QuestStatus|Race|PlayerClass|Gender)\.([A-Z_]+)', s)
        if m:
            return '"%s"' % m.group(2), 'enum'
        if t == 'field':
            raise Unsupported('field ' + s)
        if t == 'new':
            raise Unsupported('new ' + e[1])
        if t == 'call':
            return self.call(e)
        raise Unsupported(t)

    def wrap(self, node, go, op):
        if node[0] == 'bin' and op in ('&&',) and node[1] == '||':
            return '(' + go + ')'
        return go

    def paren(self, e):
        go, _ = self.ex(e)
        if e[0] in ('bin',) or (' ' in go and not go.startswith('(')):
            return '(' + go + ')'
        return go

    def obj(self, e):
        go, ty = self.ex(e)
        if ty != 'obj':
            raise Unsupported('not an object: ' + dotted(e))
        return go

    def quest(self, e):
        go, ty = self.ex(e)
        if ty != 'quest':
            raise Unsupported('not a quest state: ' + dotted(e))
        return go

    def num(self, e):
        go, ty = self.ex(e)
        if ty not in ('int', 'num', 'long'):
            raise Unsupported('not a number: ' + dotted(e))
        return go

    def call(self, e):
        _, recv, name, args = e
        r = dotted(recv) if recv is not None else ''
        rt = None
        if recv is not None and r != '?' and r.split('.')[0] in self.locals:
            try:
                rt = self.ex(recv)[1]
            except Unsupported:
                pass
        a = args
        # quest state
        if rt == 'quest' or r in ('qs',):
            q = self.quest(recv)
            if name == 'getStatus' and not a:
                return q + '.Status', 'enum'
            if name == 'getQuestVarById':
                return 'questVar(%s.Vars, %s)' % (q, self.num(a[0])), 'int'
            if name == 'getQuestVars' and not a:
                return q, 'questvars'
            if name == 'getCompliteCount':
                return q + '.CompleteCount', 'int'
            if name == 'setQuestVarById':
                return 'c.jSetVarByID(%s, %s, %s)' % (q, self.num(a[0]), self.num(a[1])), 'void'
            if name == 'setQuestVar':
                return 'c.jSetVar(%s, %s)' % (q, self.num(a[0])), 'void'
            if name == 'setStatus':
                return 'c.jSetStatus(%s, %s)' % (q, self.x(a[0])), 'void'
            raise Unsupported('qs.' + name)
        if name == 'getQuestVars' and recv is not None and self.ex(recv)[1] == 'questvars':
            return self.x(recv) + '.Vars', 'int'
        if r == 'player.getQuestStateList()' and name == 'getQuestState':
            return 'p.quest(%s)' % self.num(a[0]), 'quest'
        if recv is not None and name == 'getObjectId' and not a:
            return self.obj(recv) + '.id', 'int'
        if recv is not None and name == 'getNpcId' and not a:
            return self.obj(recv) + '.npc.ID', 'int'
        if r == 'player.getInventory()' and name == 'getItemCountByItemId':
            return 'c.s.countItems(p, %s)' % self.num(a[0]), 'long'
        if recv is None and re.search(r'private\s+(void|boolean)\s+%s\s*\(' % name, self.src):
            return self.helper(name, a)
        if recv is None or r == 'this':
            if name == 'sendQuestDialog':
                return 'c.jPage(%s, script.ID, uint16(%s))' % (self.num(a[1]), self.num(a[2])), 'bool'
            if name == 'updateQuestStatus':
                return 'c.jUpdate(%s)' % self.quest(a[1]), 'void'
            if name in ('defaultQuestStartDialog', 'defaultQuestEndDialog'):
                return 'c.%s(o, script, d)' % name, 'bool'
        if r == 'QuestService' and name == 'startQuest':
            if dotted(a[1]) == 'QuestStatus.LOCKED' and a[0][0] == 'new' and a[0][1] == 'QuestEnv':
                return 'c.jStartLocked(%s)' % self.num(a[0][2][2]), 'void'
            if dotted(a[0]) != 'env' or dotted(a[1]) != 'QuestStatus.START':
                raise Unsupported('startQuest form')
            return 'c.beginQuest(script)', 'bool'
        if r == 'QuestService' and name == 'questFinish' and len(a) == 1 and dotted(a[0]) == 'env':
            return 'c.jQuestFinish(script, d)', 'bool'
        if r == 'QuestService' and name == 'questFinish' and len(a) == 2 and dotted(a[0]) == 'env':
            return 'c.questFinish(script, uint16(d), int(%s))' % self.num(a[1]), 'bool'
        if r == 'ItemService' and name == 'decreaseKinah' and dotted(a[0]) == 'player':
            return 'c.s.decreaseKinah(p, int64(%s))' % self.num(a[1]), 'bool'
        if r == 'log' and name in ('info', 'debug', 'warn'):
            return '', 'nothing'
        if r == 'QuestService' and name == 'collectItemCheck':
            if dotted(a[0]) != 'env':
                raise Unsupported('collectItemCheck form')
            if dotted(a[1]) == 'true':
                return 'c.collectQuestItems(script.ID)', 'bool'
            return 'c.s.hasQuestItems(p, c.s.data.Quests[script.ID])', 'bool'
        if r == 'ItemService' and name == 'addItems':
            items = a[1]
            if items[0] == 'call' and items[2] == 'singletonList':
                items = items[3][0]
            if items[0] != 'new' or items[1] != 'QuestItems':
                raise Unsupported('addItems form')
            return 'c.addQuestItems([]data.QuestItem{{ID: %s, Count: %s}})' % (self.num(items[2][0]), self.num(items[2][1])), 'bool'
        if r == 'ItemService' and name == 'removeItemFromInventoryByItemId':
            return 'c.jRemoveAll(%s)' % self.num(a[1]), 'bool'
        if r == 'ItemService' and name == 'decreaseItemCountByItemId':
            return 'c.s.removeItemsByID(p, %s, int64(%s))' % (self.num(a[1]), self.num(a[2])), 'bool'
        if r == 'QuestService' and name == 'addNewSpawn':
            n = [self.x(x) for x in a[:7]]
            if n[1] == '1':
                n[1] = '0'  # Java's default instance
            return 'c.jAddNewSpawn(%s, %s, %s, float32(%s), float32(%s), float32(%s), byte(%s))' % tuple(n), 'obj'
        if r == 'MathUtil' and name == 'getDistance' and len(a) == 6:
            return 'jDistance(%s)' % ', '.join('float32(%s)' % self.x(x) for x in a), 'num'
        if r == 'InstanceService' and name == 'getNextAvailableInstance':
            return 'c.s.newInstance(%s)' % self.num(a[0]), 'inst'
        if r == 'InstanceService' and name == 'registerPlayerWithInstance' and dotted(a[1]) == 'player':
            return '%s.registered[p.ID] = true' % self.x(a[0]), 'void'
        if recv is not None and recv[0] == 'call' and recv[2] == 'getController' and name in ('onDie', 'onDespawn', 'scheduleRespawn'):
            o = self.obj(recv[1])
            if name == 'onDie':
                if dotted(a[0]) != 'null':
                    raise Unsupported('onDie(attacker)')
                return 'c.s.npcDied(%s, nil)' % o, 'void'
            if name == 'onDespawn':
                return 'c.s.despawnNpc(%s, %s)' % (o, self.x(a[0])), 'void'
            return 'c.s.scheduleRespawn(%s)' % o, 'void'
        if recv is not None and name == 'addDamage' and recv[0] == 'call' and recv[2] == 'getAggroList' and dotted(a[0]) == 'player':
            return 'c.s.addDamage(%s, p, %s)' % (self.obj(recv[1]), self.num(a[1])), 'void'
        if recv is not None and name in ('getX', 'getY', 'getZ', 'getHeading') and not a and self.ex(recv)[1] == 'obj':
            return '%s.%s' % (self.obj(recv), {'getX': 'x', 'getY': 'y', 'getZ': 'z', 'getHeading': 'heading'}[name]), 'num'
        if recv is not None and name == 'getInstanceId' and not a and self.ex(recv)[1] == 'inst':
            return self.x(recv) + '.id', 'int'
        if r == 'QuestService' and name == 'checkLevelRequirement':
            return 'int32(p.level) >= int32(c.s.data.Quests[%s].MinLevel)' % self.num(a[0]), 'bool'
        if name == 'useSkill' and not a and recv is not None and recv[0] == 'call' and recv[2] == 'getSkill' and dotted(recv[1]) == 'SkillEngine.getInstance()':
            g = recv[3]
            if dotted(g[0]) != 'player' or dotted(g[3]) != 'player':
                raise Unsupported('skill on another target')
            return 'c.jUseSkill(%s, %s)' % (self.num(g[1]), self.num(g[2])), 'void'
        if r == 'env' and name == 'setQuestId':
            return '', 'nothing'
        if r == 'String' and name == 'valueOf':
            return 'fmt.Sprint(%s)' % self.x(a[0]), 'str'
        if r == 'player.getInventory().getKinahItem()' and name == 'getItemCount':
            return 'p.kinah.Count', 'long'
        m = re.fullmatch(r'WorldMapType\.([A-Z_0-9]+)', r)
        if m and name == 'getId':
            java = open(os.path.join(ROOT, '..', 'java', 'AL-Game', 'src', 'main', 'java', 'com', 'aionemu', 'gameserver', 'world', 'WorldMapType.java')).read()
            return re.search(r'\b%s\((\d+)' % m.group(1), java).group(1), 'int'
        if r == 'PacketSendUtility' and name == 'sendPacket' and dotted(a[0]) == 'player' and a[1][0] == 'new' and a[1][1] == 'SM_EMOTION' and dotted(a[1][2][0]) == 'player':
            e = a[1][2]
            return 'c.send(c.s.playerEmotionTo(p, %s, %s, %s, 0, 0, 0, 0))' % (emotion(e[1]), self.num(e[2]), self.num(e[3])), 'void'
        if r == 'player' and name == 'isTargeting':
            return 'p.targetID == %s' % self.num(a[0]), 'bool'
        if r == 'ThreadPoolManager.getInstance()' and name == 'schedule':
            if a[0][0] != 'runnable':
                raise Unsupported('schedule of ' + a[0][0])
            ind = self.ind
            for field in a[0][2]:
                self.pre.extend(self.stmt(field, 0))
            self.ind = ind
            self.closure += 1
            body = self.stmts(a[0][1][1], self.ind + 1)
            self.closure -= 1
            pad = '\t' * self.ind
            return 'c.jLater(%s, func() {\n%s\n%s})' % (self.num(a[1]), '\n'.join(body), pad), 'void'
        if r == 'PacketSendUtility' and name == 'sendMessage' and dotted(a[0]) == 'player':
            return 'c.s.tell(p, %s)' % self.x(a[1]), 'void'
        if r == 'PacketSendUtility' and name == 'broadcastPacket':
            pk = a[1]
            if pk[0] != 'new' or pk[1] != 'SM_EMOTION' or len(pk[2]) != 4:
                raise Unsupported('broadcast of ' + dotted(pk))
            kind = emotion(pk[2][1])
            if dotted(a[0]) == 'player' and dotted(pk[2][0]) == 'player':
                return 'p.broadcast(c.s.playerEmotionTo(p, %s, %s, %s, 0, 0, 0, 0), true)' % (kind, self.num(pk[2][2]), self.num(pk[2][3])), 'void'
            if dotted(a[0]) == 'player.getTarget()':
                return 'c.jTargetEmotion(%s, %s, %s)' % (kind, self.num(pk[2][2]), self.num(pk[2][3])), 'void'
            raise Unsupported('emotion broadcast from ' + dotted(a[0]))
        if r == 'TeleportService' and name == 'teleportTo' and dotted(a[0]) == 'player':
            n = [self.x(x) for x in a[1:]]
            if len(n) == 5:  # world, x, y, z, delay
                return 'c.s.teleportTo(p, %s, %s, %s, %s, byte(p.Heading), %s*time.Millisecond)' % (n[0], n[1], n[2], n[3], 'time.Duration(%s)' % n[4]), 'bool'
            if len(n) in (6, 7) and n[1] == '1':
                n[1] = '0'  # Java's default instance
            if len(n) == 6:  # world, instance, x, y, z, delay (a (byte) heading here is Java picking the int delay)
                return 'c.jTeleport(%s, %s, %s, %s, %s, byte(p.Heading), int32(%s))' % tuple(n), 'bool'
            if len(n) == 7:
                return 'c.jTeleport(%s, %s, %s, %s, %s, %s, int32(%s))' % tuple(n), 'bool'
            raise Unsupported('teleportTo form')
        if recv is not None and name in ('onDelete', 'delete') and recv[0] == 'call' and recv[2] == 'getController':
            return 'c.s.despawnNpc(%s, true)' % self.obj(recv[1]), 'void'
        if r == 'PacketSendUtility' and name == 'sendPacket' and dotted(a[0]) == 'player' and a[1][0] == 'new' and a[1][1] == 'SM_USE_OBJECT':
            u = a[1][2]
            if u[2] != ('num', '3000'):
                raise Unsupported('use time')
            return 'c.send(useObject(%s, %s, byte(%s)))' % (self.num(u[0]), self.num(u[1]), self.num(u[3])), 'void'
        if r == 'PacketSendUtility' and name == 'sendPacket' and dotted(a[0]) == 'player':
            pk = a[1]
            if pk[0] != 'new':
                raise Unsupported('sendPacket of ' + dotted(pk))
            if pk[1] == 'SM_DIALOG_WINDOW':
                quest = self.num(pk[2][2]) if len(pk[2]) > 2 else '0'
                return 'c.send(dialogWindow(%s, uint16(%s), %s))' % (self.num(pk[2][0]), self.num(pk[2][1]), quest), 'void'
            if pk[1] == 'SM_PLAY_MOVIE' and len(pk[2]) == 2 and pk[2][0] in (('num', '0'), ('num', '1')):
                return 'c.send(movie(%s, uint16(%s)))' % (pk[2][0][1], self.num(pk[2][1])), 'void'
            raise Unsupported('packet ' + pk[1])
        raise Unsupported('call %s.%s' % (r, name))

    def helper(self, name, args):
        """A private method of the handler class, translated once as its own Go method."""
        m = re.search(r'private\s+(void|boolean)\s+%s\s*\(([^)]*)\)\s*\{' % name, self.src)
        ret, params = m.group(1), [x.split() for x in m.group(2).split(',') if x.strip()]
        params = [x[1:] if x[0] == 'final' else x for x in params]
        gofn = 'java%d%s' % (self.qid, name[0].upper() + name[1:])
        if name not in self.helpers:
            self.helpers[name] = ''
            sub = Emitter(self.qid, self.src)
            sub.helpers = self.helpers
            goparams = []
            for jtype, pname in params:
                if jtype == 'QuestEnv':
                    if pname != 'env':
                        raise Unsupported('env parameter name')
                    continue
                if jtype not in ('int', 'long', 'boolean'):
                    raise Unsupported('helper parameter ' + jtype)
                sub.locals[pname] = {'int': 'int', 'long': 'long', 'boolean': 'bool'}[jtype]
                goparams.append('%s %s' % (goname(pname), {'int': 'int32', 'long': 'int64', 'boolean': 'bool'}[jtype]))
            tree = Parser(tokenize(method_body(self.src, name, ret))).block()
            body = sub.stmts(tree[1], 1)
            if ret == 'boolean' and not terminates(tree):
                body.append('\treturn false')
            self.helpers[name] = '\n'.join(['// %s is the handler\'s private %s.' % (gofn, name),
                'func (c *conn) %s(o *object, script *data.QuestScript, d int32%s)%s {' % (gofn, ''.join(', ' + x for x in goparams), ' bool' if ret == 'boolean' else ''),
                '\tp := c.player', '\t_ = p'] + body + ['}']) + '\n'
        call_args = [self.x(x) for (jtype, _), x in zip(params, args) if jtype != 'QuestEnv']
        return 'c.%s(o, script, d%s)' % (gofn, ''.join(', ' + x for x in call_args)), ('bool' if ret == 'boolean' else 'void')

    # statements
    def stmts(self, body, ind):
        out = []
        for s in body:
            saved, self.pre = self.pre, []
            lines = self.stmt(s, ind)
            out.extend('\t' * ind + x for x in self.pre)
            out.extend(lines)
            self.pre = saved
        return out

    def stmt(self, s, ind):
        self.ind = ind
        pad = '\t' * ind
        t = s[0]
        if t == 'block':
            return self.stmts(s[1], ind)
        if t == 'return':
            if s[1] is None:
                if not self.closure:
                    raise Unsupported('bare return')
                return [pad + 'return']
            go, ty = self.ex(s[1])
            if ty not in ('bool',):
                raise Unsupported('return of ' + ty)
            return [pad + 'return ' + go]
        if t == 'break':
            return [pad + 'break']
        if t == 'continue':
            return [pad + 'continue']
        if t == 'array':
            self.locals[s[1]] = 'ints'
            return [pad + '%s := []int32{%s}' % (goname(s[1]), ', '.join(self.num(x) for x in s[2]))]
        if t == 'foreach':
            name, seq, body = s[1], s[2], s[3]
            if dotted(seq) == 'player.getKnownList().getKnownObjects().values()':
                self.locals[name] = 'obj'
                return [pad + 'for _, %s := range p.seen {' % goname(name)] + self.stmt(body, ind + 1) + [pad + '}']
            go, ty = self.ex(seq)
            if ty != 'ints':
                raise Unsupported('for over ' + ty)
            self.locals[name] = 'int'
            return [pad + 'for _, %s := range %s {' % (goname(name), go)] + self.stmt(body, ind + 1) + [pad + '}']
        if t == 'expr':
            go, ty = self.ex(s[1])
            if ty == 'nothing':
                return []
            if ty in ('bool', 'obj', 'inst') and go.startswith(('c.java', 'c.jAddNewSpawn', 'c.s.newInstance', 'c.s.teleportTo', 'c.jTeleport', 'c.s.decreaseKinah', 'c.jQuestFinish', 'c.questFinish', 'c.addQuestItems', 'c.jRemoveAll', 'c.beginQuest', 'c.collectQuestItems', 'c.s.removeItemsByID', 'c.jPage', 'c.default')):
                return [pad + go]
            if ty != 'void':
                raise Unsupported('expression statement ' + go)
            return [pad + line if i == 0 else line for i, line in enumerate(go.split('\n'))]
        if t == 'decl':
            jtype, name, init = s[1], s[2], s[3]
            # the usual preamble
            if jtype == 'Player':
                if init is not None and dotted(init) != 'env.getPlayer()':
                    raise Unsupported('player local')
                return []
            if jtype == 'WorldMapInstance':
                self.locals[name] = 'inst'
                return [pad + '%s := %s' % (goname(name), self.x(init)), pad + '_ = %s' % goname(name)]
            if jtype == 'Npc' and init is not None and dotted(init) == 'player.getTarget()':
                self.locals[name] = 'obj'
                return [pad + '%s := c.jTarget()' % goname(name), pad + '_ = %s' % goname(name)]
            if jtype == 'Npc':
                if init is not None and dotted(init) != 'env.getVisibleObject()':
                    raise Unsupported('npc local')
                self.locals[name] = 'obj'
                return [pad + '%s := o' % goname(name), pad + '_ = %s' % goname(name)]
            if jtype == 'QuestState':
                self.locals[name] = 'quest'
                go = self.quest(init) if init is not None else 'nil'
                if init is None:
                    return [pad + 'var %s *store.Quest' % goname(name), pad + '_ = %s' % goname(name)]
                return [pad + '%s := %s' % (goname(name), go), pad + '_ = %s' % goname(name)]
            if jtype in ('int', 'long', 'boolean'):
                kind = {'int': 'int', 'long': 'long', 'boolean': 'bool'}[jtype]
                gotype = {'int': 'int32', 'long': 'int64', 'boolean': 'bool'}[jtype]
                self.locals[name] = kind
                if init is None:
                    return [pad + 'var %s %s' % (goname(name), gotype), pad + '_ = %s' % goname(name)]
                go, ty = self.ex(init)
                if kind == 'int' and ty == 'long':
                    go = 'int32(%s)' % go
                return [pad + 'var %s %s = %s' % (goname(name), gotype, go), pad + '_ = %s' % goname(name)]
            if jtype in ('PlayerClass', 'Race', 'Gender'):
                self.locals[name] = 'enum'
                return [pad + '%s := %s' % (goname(name), self.x(init)), pad + '_ = %s' % goname(name)]
            if jtype == 'QuestTemplate':
                raise Unsupported('QuestTemplate local')
            raise Unsupported('local of type ' + jtype)
        if t == 'assign':
            target, op, value = s[1], s[2], s[3]
            if target[0] != 'name' or target[1] not in self.locals:
                raise Unsupported('assignment to ' + dotted(target))
            go, ty = self.ex(value)
            kind = self.locals[target[1]]
            if kind == 'int' and ty == 'long':
                go = 'int32(%s)' % go
            return [pad + '%s %s %s' % (goname(target[1]), op, go)]
        if t == 'if':
            cond, then, other = s[1], s[2], s[3]
            if cond == ('true',) or self.x(cond) == 'true':
                return self.stmt(then, ind)
            out = [pad + 'if %s {' % self.x(cond)]
            out += self.stmt(then, ind + 1)
            while other is not None and other[0] == 'if' and other[1] != ('true',):
                out.append(pad + '} else if %s {' % self.x(other[1]))
                out += self.stmt(other[2], ind + 1)
                other = other[3]
            if other is not None:
                out.append(pad + '} else {')
                out += self.stmt(other, ind + 1)
            out.append(pad + '}')
            return out
        if t == 'switch':
            subject, cases = s[1], s[2]
            # merge empty labels into the next case
            merged = []
            pending = []
            for labels, body in cases:
                if labels is None:
                    pending.append(None)
                else:
                    pending.extend(labels)
                if body:
                    merged.append((pending, body))
                    pending = []
            if pending:
                merged.append((pending, []))
            out = [pad + 'switch %s {' % self.x(subject)]
            for i, (labels, body) in enumerate(merged):
                if None in labels:
                    if len(labels) > 1:
                        # "case X: default:" is just default
                        pass
                    out.append(pad + 'default:')
                elif self.ex(subject)[1] == 'enum':
                    out.append(pad + 'case %s:' % ', '.join('"%s"' % l[1] if l[0] == 'name' else self.x(l) for l in labels))
                else:
                    out.append(pad + 'case %s:' % ', '.join(self.num(l) for l in labels))
                body = list(body)
                ends_break = body and body[-1] == ('break',)
                if ends_break:
                    body = body[:-1]
                out += self.stmts(body, ind + 1)
                if not ends_break and not terminates(('block', body)):
                    if i == len(merged) - 1:
                        pass  # falls out of the switch
                    else:
                        out.append(pad + '\tfallthrough')
            out.append(pad + '}')
            return out
        raise Unsupported(t)


def emotion(e):
    m = re.fullmatch(r'EmotionType\.([A-Z_0-9]+)', dotted(e))
    if not m:
        raise Unsupported('emotion ' + dotted(e))
    java = open(os.path.join(ROOT, '..', 'java', 'AL-Game', 'src', 'main', 'java', 'com', 'aionemu', 'gameserver', 'model', 'EmotionType.java')).read()
    value = re.search(r'\b%s\((-?\d+)\)' % m.group(1), java).group(1)
    gosrc = open(os.path.join(ROOT, 'game', 'emotions.go')).read()
    g = re.search(r'\n\t(emote\w+)\s*=\s*%s\n' % value, gosrc)
    if not g:
        raise Unsupported('no Go emotion %s' % m.group(1))
    return g.group(1)


def goname(n):
    return {'var': 'var_', 'type': 'type_', 'func': 'func_', 'range': 'range_', 'map': 'map_', 'go': 'go_'}.get(n, n)


def terminates(s):
    """Go's terminating statement rule, on the Java tree."""
    t = s[0]
    if t == 'return':
        return True
    if t == 'block':
        body = [x for x in s[1] if x != ('block', [])]
        return bool(body) and terminates(body[-1])
    if t == 'if':
        if s[1] == ('true',):
            return terminates(s[2])
        return s[3] is not None and terminates(s[2]) and terminates(s[3])
    if t == 'switch':
        cases = s[2]
        if not any(labels is None for labels, _ in cases):
            return False
        for labels, body in cases:
            if any(has_break(x) for x in body):
                return False
        # every case with statements must terminate (or fall through into one that does)
        last = cases[-1][1]
        return bool(last) and terminates(('block', last)) and all(
            not body or terminates(('block', body)) or True for _, body in cases)
    return False


def has_break(s):
    t = s[0]
    if t == 'break':
        return True
    if t == 'block':
        return any(has_break(x) for x in s[1])
    if t == 'if':
        return has_break(s[2]) or (s[3] is not None and has_break(s[3]))
    return False


def java_file(qid):
    files = glob.glob(os.path.join(JAVA, '*', '_%d[A-Za-z]*.java' % qid))
    if len(files) != 1:
        raise Unsupported('no single Java file for %d' % qid)
    return files[0]


def method_body(src, name, ret=None):
    if ret is not None:
        m = re.search(r'private\s+%s\s+%s\s*\([^)]*\)\s*\{' % (ret, name), src)
    else:
        m = re.search(r'public\s+boolean\s+%s\s*\(\s*QuestEnv\s+\w+\s*\)\s*\{' % name, src)
    if not m:
        raise Unsupported('no ' + name)
    depth, i = 1, m.end()
    while depth:
        ch = src[i]
        if ch == '{':
            depth += 1
        elif ch == '}':
            depth -= 1
        elif ch == '"':
            i = src.index('"', i + 1)
        elif src.startswith('//', i):
            i = src.index('\n', i)
        elif src.startswith('/*', i):
            i = src.index('*/', i) + 1
        i += 1
    return src[m.end() - 1:i]


def translate(qid):
    path = java_file(qid)
    src = open(path).read()
    if re.search(r'onDialogEvent\s*\(\s*QuestEnv\s+(\w+)', src).group(1) != 'env':
        raise Unsupported('env name')
    tree = Parser(tokenize(method_body(src, 'onDialogEvent'))).block()
    em = Emitter(qid, src)
    body = em.stmts(tree[1], 1)
    if not terminates(tree):
        body.append('\treturn false')
    rel = os.path.relpath(path, os.path.join(ROOT, '..'))
    m = re.search(r'void\s+register\s*\(\s*\)\s*\{(.*?)\n\s*\}', src, re.S)
    talks = [int(n) for n in re.findall(r'setNpcQuestData\((\d+)\)\.addOnTalkEvent\(questId\)', m.group(1))] if m else []
    lines = ['// javaDialog%d is %s onDialogEvent.' % (qid, rel),
             '// talk npcs: %s' % ' '.join(str(n) for n in talks),
             'func (c *conn) javaDialog%d(o *object, script *data.QuestScript, d int32) bool {' % qid,
             '\tp := c.player',
             '\t_ = p']
    lines += body
    lines.append('}')
    out = '\n'.join(lines) + '\n'
    for helper in em.helpers.values():
        out += '\n' + helper
    level_up = translate_level_up(qid)
    if level_up:
        out += '\n' + level_up
    return out


def translate_level_up(qid):
    """onLvlUpEvent of the handlers that unlock a LOCKED quest (the campaign chains); None for any other."""
    src = open(java_file(qid)).read()
    if not re.search(r'addQuestLvlUp\(questId\)', src):
        return None
    body = method_body(src, 'onLvlUpEvent')
    if 'LOCKED' not in body:
        return None
    tree = Parser(tokenize(body)).block()
    em = Emitter(qid)
    lines = em.stmts(tree[1], 1)
    if not terminates(tree):
        lines.append('\treturn false')
    return '\n'.join(['// javaLevelUp%d is its onLvlUpEvent.' % qid,
                      'func (c *conn) javaLevelUp%d(script *data.QuestScript) bool {' % qid,
                      '\tp := c.player', '\t_ = p'] + lines + ['}']) + '\n'


HEADER = '''// Code generated by scripts/quest-java-port.py; DO NOT EDIT.

package game

import (
	"fmt"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

var (
	_ *store.Quest
	_ time.Duration
	_ = fmt.Sprint
)

'''


def load_existing():
    if not os.path.exists(OUT):
        return {}
    src = open(OUT).read()
    funcs = {}
    marks = [m for m in re.finditer(r'^// java(?:Dialog|LevelUp)(\d+) is ', src, re.M)]
    for i, m in enumerate(marks):
        end = marks[i + 1].start() if i + 1 < len(marks) else len(src)
        block = src[m.start():end].rstrip('\n') + '\n'
        qid = int(m.group(1))
        funcs[qid] = (funcs[qid] + '\n' + block) if qid in funcs else block
    return funcs


def write(funcs):
    ids = sorted(funcs)
    out = HEADER
    out += '// javaDialogs are the Java onDialogEvent handlers translated one to one.\n'
    out += 'var javaDialogs = map[int32]func(c *conn, o *object, script *data.QuestScript, d int32) bool{\n'
    for i in ids:
        if 'func (c *conn) javaDialog%d(' % i in funcs[i]:
            out += '\t%d: (*conn).javaDialog%d,\n' % (i, i)
    out += '}\n\n'
    talks = {}
    for i in ids:
        m = re.search(r'// talk npcs: ([\d ]*)', funcs[i])
        for n in (m.group(1).split() if m else []):
            talks.setdefault(int(n), []).append(i)
    out += '// javaLevelUps are the translated onLvlUpEvent handlers that unlock a LOCKED quest.\n'
    out += 'var javaLevelUps = map[int32]func(c *conn, script *data.QuestScript) bool{\n'
    for i in ids:
        if 'func (c *conn) javaLevelUp%d(' % i in funcs[i]:
            out += '\t%d: (*conn).javaLevelUp%d,\n' % (i, i)
    out += '}\n\n'
    out += '// javaTalkNPCs are the npcs each translated handler registers its talk event on (register()).\n'
    out += 'var javaTalkNPCs = map[int32][]int32{\n'
    for n in sorted(talks):
        out += '\t%d: {%s},\n' % (n, ', '.join(str(i) for i in talks[n]))
    out += '}\n'
    for i in ids:
        out += '\n' + funcs[i]
    open(OUT, 'w').write(out)


def main(argv):
    check = '--check' in argv
    ids = [int(a) for a in argv if a.isdigit()]
    if check:
        for i in ids:
            try:
                print(translate(i))
            except Unsupported as e:
                print('%d: cannot translate: %s' % (i, e))
        return 0
    funcs = load_existing()
    failed = []
    level_only = '--level-up-only' in argv
    for i in ids:
        if level_only:
            level_up = translate_level_up(i)
            if level_up:
                old = funcs.get(i, '')
                cut = old.find('// javaLevelUp')
                keep = old if cut < 0 else old[:cut]
                funcs[i] = (keep.rstrip('\n') + '\n\n' + level_up) if keep.strip() else level_up
            continue
        try:
            funcs[i] = translate(i)
        except Unsupported as e:
            failed.append((i, str(e)))
            try:
                level_up = translate_level_up(i)
            except Unsupported:
                level_up = None
            if level_up:
                funcs[i] = level_up  # the dialog stays hand-ported; the level-up unlock is translated
    write(funcs)
    print('translated %d, left for hand ports %d' % (len(ids) - len(failed), len(failed)))
    for i, why in failed:
        print('  %d: %s' % (i, why))
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
