// Command pktdiff compares two gamesniff logs (one per game server, same
// client scenario) and prints, per client request, the server packets that only
// one side sent and the same-opcode packets whose bytes differ.
//
//	pktdiff -quest java.log go.log
//	pktdiff -from CM_SHOW_DIALOG -ops SM_DIALOG_WINDOW,SM_QUEST_LIST java.log go.log
//	pktdiff -dialogs -label ready java.log go.log        client-action timeline, verdict per step
//	pktdiff -gen-test 1001 -items 182200001 java.log ready   Go replay test from a recorded Java run
//
// Log lines are gamesniff's `msg=server|client op=NAME size=N hex=...`, or the
// testdata form `server|client NAME N hex` (an optional time prefix is ignored).
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	from := flag.String("from", "", "start each log at the first client packet with this opcode (e.g. CM_SHOW_DIALOG)")
	until := flag.String("until", "", "stop each log after the last client packet with this opcode")
	ops := flag.String("ops", "", "compare only these server opcodes (comma separated)")
	quest := flag.Bool("quest", false, "shorthand for -ops with the quest-related opcodes")
	skip := flag.String("skip", "", "ignore these server opcodes (e.g. SM_MOVE,SM_NPC_INFO)")
	norm := flag.String("norm", "id,time", "volatile fields to mask: id, time, coord, text (none to disable)")
	all := flag.Bool("all", false, "also print requests without differences")
	last := flag.Bool("last-session", false, "start each log at its last CM_ENTER_WORLD (logs often hold several sessions)")
	label := flag.String("label", "", "use only the packets after this mark (quest-debug.sh mark <label>) up to the next mark, in the first log")
	labelB := flag.String("label2", "", "same for the second log (default: -label); lets one log hold both runs")
	dialogs := flag.Bool("dialogs", false, "render both logs as a client-action timeline (dialog clicks/selects, movie ends, kills, questions) with the server effects, a verdict per step and the first divergence")
	genTest := flag.Int("gen-test", 0, "with a quest id: `pktdiff -gen-test <id> [-items a,b] [-o file] <java.log> [label]` writes game/quest_<id>_replay_test.go replaying the Java run's client actions against the Go handlers")
	items := flag.String("items", "", "-gen-test: quest item ids granted where the run looted them (comma separated)")
	out := flag.String("o", "", "-gen-test: output file (default game/quest_<id>_replay_test.go; - for stdout)")
	flag.Parse()
	if *genTest != 0 {
		genTestMain(*genTest, *items, *out, flag.Args())
		return
	}
	if *labelB == "" {
		*labelB = *label
	}
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: pktdiff [flags] <java.log> <go.log>")
		os.Exit(2)
	}
	options := Options{From: *from, Until: *until, Only: split(*ops), Skip: split(*skip), Norm: split(*norm), All: *all}
	if *quest && !*dialogs { // dialogs mode has its own quest effect set (-ops/-skip still narrow it)
		options.Only = append(options.Only, QuestOpcodes...)
	}
	java, err := ReadLog(flag.Arg(0), *label)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	goLog, err := ReadLog(flag.Arg(1), *labelB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *last {
		java, goLog = lastSession(java), lastSession(goLog)
	}
	if *dialogs {
		if Dialogs(os.Stdout, java, goLog, options) {
			os.Exit(1)
		}
		return
	}
	if Compare(os.Stdout, java, goLog, options) {
		os.Exit(1)
	}
}

func genTestMain(questID int, items, out string, args []string) {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: pktdiff -gen-test <questId> [-items a,b] [-o file] <java.log> [label]")
		os.Exit(2)
	}
	label := ""
	if len(args) == 2 && args[1] != "-" {
		label = args[1]
	}
	packets, err := ReadLog(args[0], label)
	if err == nil && len(packets) == 0 {
		err = fmt.Errorf("no packets in %s%s", args[0], map[bool]string{true: " after mark " + label}[label != ""])
	}
	var ids []int32
	for _, item := range split(items) {
		var id int32
		if _, scanErr := fmt.Sscan(item, &id); scanErr != nil {
			err = fmt.Errorf("bad item id %q", item)
		}
		ids = append(ids, id)
	}
	var buf bytes.Buffer
	if err == nil {
		source := filepath.Base(args[0])
		if label != "" {
			source += " " + label
		}
		err = GenTest(&buf, int32(questID), packets, source, ids)
	}
	if err == nil {
		if out == "" {
			out = fmt.Sprintf("game/quest_%d_replay_test.go", questID)
		}
		if out == "-" {
			_, err = os.Stdout.Write(buf.Bytes())
		} else if err = os.WriteFile(out, buf.Bytes(), 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "wrote %s; run: go test ./game -run TestQuest%dJavaReplay\n", out, questID)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// lastSession returns the packets from the last CM_ENTER_WORLD on (all of them when there is none).
func lastSession(packets []Packet) []Packet {
	for i := len(packets) - 1; i >= 0; i-- {
		if packets[i].Client && packets[i].Op == "CM_ENTER_WORLD" {
			return packets[i:]
		}
	}
	return packets
}

func split(list string) []string {
	var out []string
	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" && item != "none" {
			out = append(out, item)
		}
	}
	return out
}
