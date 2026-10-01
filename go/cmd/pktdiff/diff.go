package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

// QuestOpcodes are the server opcodes a quest scenario touches.
var QuestOpcodes = []string{
	"SM_QUEST_ACTION", "SM_QUEST_LIST", "SM_QUEST_ACCEPTED", "SM_DIALOG_WINDOW", "SM_SYSTEM_MESSAGE",
	"SM_NEARBY_QUESTS", "SM_PLAY_MOVIE", "SM_EMOTION", "SM_MOVE", "SM_MESSAGE", "SM_ADD_ITEMS",
	"SM_UPDATE_ITEM", "SM_DELETE_ITEM", "SM_STATUPDATE_EXP", "SM_LEVEL_UPDATE", "SM_SHOW_NPC_ON_MAP",
	"SM_TARGET_SELECTED", "SM_TARGET_UPDATE", "SM_USE_OBJECT", "SM_ITEM_USAGE_ANIMATION",
	"SM_LOOT_ITEMLIST", "SM_LOOT_STATUS",
}

// Options selects and normalises what is compared.
type Options struct {
	From, Until string
	Only, Skip  []string
	Norm        []string
	All         bool
}

// Packet is one line of a sniff log. Body is the packet after its 3-byte header.
type Packet struct {
	Client bool
	Op     string
	Body   []byte
}

var lineRE = regexp.MustCompile(`\b(server|client)\s+(?:op=)?(\w*)\s+(?:size=)?\d+\s+(?:hex=)?([0-9a-fA-F]*)`)

// markRE is gamesniff's marker line (quest-debug.sh mark <label>).
var markRE = regexp.MustCompile(`\bmsg=mark label=(\S+)`)

// ReadLog parses a gamesniff log file; with a label, only what follows that
// mark up to the next mark of any kind.
func ReadLog(path, label string) ([]Packet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if label != "" {
		return parseLog(f, label)
	}
	return ParseLog(f)
}

// ParseLog parses sniff lines from r; other lines are ignored.
func ParseLog(r io.Reader) ([]Packet, error) { return parseLog(r, "") }

func parseLog(r io.Reader, label string) ([]Packet, error) {
	var packets []Packet
	inside := label == ""
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		if mark := markRE.FindStringSubmatch(scanner.Text()); mark != nil && label != "" {
			inside = mark[1] == label
			continue
		}
		m := lineRE.FindStringSubmatch(scanner.Text())
		if m == nil || !inside {
			continue
		}
		body, err := hex.DecodeString(m[3])
		if err != nil {
			continue
		}
		op := m[2]
		if op == "" {
			op = "UNKNOWN"
		}
		packets = append(packets, Packet{Client: m[1] == "client", Op: op, Body: body})
	}
	return packets, scanner.Err()
}

// turn is a client request and the server packets that followed it.
type turn struct {
	request *Packet
	replies []Packet
}

func (t turn) name() string {
	if t.request == nil {
		return "(before the first request)"
	}
	return t.request.Op
}

func window(packets []Packet, from, until string) []Packet {
	start, end := 0, len(packets)
	if from != "" {
		start = len(packets)
		for i, p := range packets {
			if p.Client && p.Op == from {
				start = i
				break
			}
		}
	}
	if until != "" {
		for i := len(packets) - 1; i >= start && i < len(packets); i-- {
			if packets[i].Client && packets[i].Op == until {
				end = i + 1
				break
			}
		}
	}
	return packets[start:end]
}

func turns(packets []Packet, options Options, mask *masker) []turn {
	only := set(options.Only)
	skip := set(options.Skip)
	out := []turn{{}}
	for i := range packets {
		p := packets[i]
		if p.Client {
			out = append(out, turn{request: &packets[i]})
			continue
		}
		p.Body = mask.apply(p.Op, p.Body) // learns ids from every packet, kept or not
		if skip[p.Op] || (len(only) > 0 && !only[p.Op]) {
			continue
		}
		out[len(out)-1].replies = append(out[len(out)-1].replies, p)
	}
	if len(out[0].replies) == 0 {
		out = out[1:]
	}
	return out
}

func set(items []string) map[string]bool {
	m := map[string]bool{}
	for _, i := range items {
		m[i] = true
	}
	return m
}

// align pairs the turns of a and b by client opcode (longest common subsequence).
// A turn with no request (server packets before the first one) pairs only with its twin.
func align(a, b []turn) [][2]int {
	names := func(ts []turn) []string {
		out := make([]string, len(ts))
		for i, t := range ts {
			out[i] = t.name()
		}
		return out
	}
	return alignNames(names(a), names(b))
}

// alignNames pairs two name sequences by longest common subsequence; -1 marks a side with no partner.
func alignNames(a, b []string) (pairs [][2]int) {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				lcs[i][j] = lcs[i+1][j+1] + 1
			case lcs[i+1][j] >= lcs[i][j+1]:
				lcs[i][j] = lcs[i+1][j]
			default:
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	for i, j := 0, 0; i < n || j < m; {
		switch {
		case i == n:
			pairs = append(pairs, [2]int{-1, j})
			j++
		case j == m:
			pairs = append(pairs, [2]int{i, -1})
			i++
		case a[i] == b[j]:
			pairs = append(pairs, [2]int{i, j})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			pairs = append(pairs, [2]int{i, -1})
			i++
		default:
			pairs = append(pairs, [2]int{-1, j})
			j++
		}
	}
	return pairs
}

type tally struct{ java, goServer, differ int }

// Compare prints the differences between the Java and Go logs and reports
// whether any were found.
func Compare(w io.Writer, java, goLog []Packet, options Options) bool {
	java = window(java, options.From, options.Until)
	goLog = window(goLog, options.From, options.Until)
	ja := turns(java, options, newMasker(java, options.Norm))
	go_ := turns(goLog, options, newMasker(goLog, options.Norm))
	totals := map[string]*tally{}
	count := func(op string) *tally {
		if totals[op] == nil {
			totals[op] = &tally{}
		}
		return totals[op]
	}
	found := false
	for _, pair := range align(ja, go_) {
		var jt, gt turn
		label := ""
		switch {
		case pair[1] < 0:
			jt, label = ja[pair[0]], fmt.Sprintf("request only in Java (java #%d)", pair[0])
		case pair[0] < 0:
			gt, label = go_[pair[1]], fmt.Sprintf("request only in Go (go #%d)", pair[1])
		default:
			jt, gt = ja[pair[0]], go_[pair[1]]
			label = fmt.Sprintf("(java #%d, go #%d)", pair[0], pair[1])
		}
		lines := compareReplies(jt.replies, gt.replies, count)
		if pair[0] < 0 || pair[1] < 0 {
			lines = append([]string{"  " + label}, lines...)
		}
		if len(lines) == 0 && !options.All {
			continue
		}
		if len(lines) > 0 {
			found = true
		}
		title := jt.name()
		if pair[0] < 0 {
			title = gt.name()
		}
		fmt.Fprintf(w, "== %s %s\n", title, label)
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}
	printSummary(w, totals, len(ja), len(go_))
	return found
}

func compareReplies(java, goServer []Packet, count func(string) *tally) (lines []string) {
	byOp := func(ps []Packet) map[string][]Packet {
		m := map[string][]Packet{}
		for _, p := range ps {
			m[p.Op] = append(m[p.Op], p)
		}
		return m
	}
	jm, gm := byOp(java), byOp(goServer)
	var ops []string
	for op := range jm {
		ops = append(ops, op)
	}
	for op := range gm {
		if _, ok := jm[op]; !ok {
			ops = append(ops, op)
		}
	}
	sort.Strings(ops)
	sameSet := true
	for _, op := range ops {
		j, g := jm[op], gm[op]
		count(op).java += len(j)
		count(op).goServer += len(g)
		for i := 0; i < len(j) || i < len(g); i++ {
			switch {
			case i >= len(g):
				lines = append(lines, fmt.Sprintf("  only Java: %s size=%d %s", op, len(j[i].Body)+3, short(j[i].Body)))
				sameSet = false
			case i >= len(j):
				lines = append(lines, fmt.Sprintf("  only Go:   %s size=%d %s", op, len(g[i].Body)+3, short(g[i].Body)))
				sameSet = false
			case string(j[i].Body) != string(g[i].Body):
				count(op).differ++
				lines = append(lines, describe(op, i, j[i].Body, g[i].Body)...)
			}
		}
	}
	if sameSet {
		if jo, gs := opSequence(java), opSequence(goServer); jo != gs {
			lines = append(lines, "  order differs:", "    java: "+jo, "    go:   "+gs)
		}
	}
	return lines
}

func opSequence(ps []Packet) string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Op
	}
	return strings.Join(names, " ")
}

func short(b []byte) string {
	if len(b) > 32 {
		return hex.EncodeToString(b[:32]) + "..."
	}
	return hex.EncodeToString(b)
}

func printSummary(w io.Writer, totals map[string]*tally, javaTurns, goTurns int) {
	var ops []string
	for op, t := range totals {
		if t.java != t.goServer || t.differ > 0 {
			ops = append(ops, op)
		}
	}
	sort.Strings(ops)
	fmt.Fprintf(w, "-- requests: java %d, go %d\n", javaTurns, goTurns)
	if len(ops) == 0 {
		fmt.Fprintln(w, "-- no server packet differs")
		return
	}
	fmt.Fprintln(w, "-- opcodes that differ: opcode, java count, go count, same-count packets with different bytes")
	for _, op := range ops {
		t := totals[op]
		fmt.Fprintf(w, "   %-28s java=%d go=%d bytesDiffer=%d\n", op, t.java, t.goServer, t.differ)
	}
}

// describe lists the byte runs where two same-opcode packets differ, with a
// field hint where the offset is known.
func describe(op string, index int, java, goServer []byte) []string {
	lines := []string{fmt.Sprintf("  differs:   %s #%d size java=%d go=%d", op, index, len(java)+3, len(goServer)+3)}
	n := min(len(java), len(goServer))
	runs := 0
	for i := 0; i < n; {
		if java[i] == goServer[i] {
			i++
			continue
		}
		start := i
		for i < n && java[i] != goServer[i] {
			i++
		}
		// Merge runs separated by fewer than 2 equal bytes.
		for i+2 < n && (java[i] != goServer[i] || java[i+1] != goServer[i+1]) {
			i++
			for i < n && java[i] != goServer[i] {
				i++
			}
		}
		if runs++; runs > 6 {
			lines = append(lines, "      ...")
			break
		}
		lines = append(lines, fmt.Sprintf("      @%d..%d java=%s go=%s%s", start, i-1, hex.EncodeToString(java[start:i]), hex.EncodeToString(goServer[start:i]), fieldHint(op, start, i)))
	}
	if len(java) != len(goServer) {
		lines = append(lines, fmt.Sprintf("      tail from @%d: java=%s go=%s", n, short(java[n:]), short(goServer[n:])))
	}
	if jt, gt := text(java), text(goServer); jt != gt {
		lines = append(lines, fmt.Sprintf("      text: java=%q go=%q", jt, gt))
	}
	return lines
}

// text returns the longest UTF-16LE printable run of at least 3 characters.
func text(b []byte) string {
	best, cur := "", []uint16{}
	flush := func() {
		if s := string(utf16.Decode(cur)); len(cur) >= 3 && len(s) > len(best) {
			best = s
		}
		cur = cur[:0]
	}
	for i := 0; i+1 < len(b); i += 2 {
		if c := binary.LittleEndian.Uint16(b[i:]); c >= 0x20 && c < 0x7f {
			cur = append(cur, c)
		} else {
			flush()
		}
	}
	flush()
	return best
}
