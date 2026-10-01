package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parse(t *testing.T, log string) []Packet {
	t.Helper()
	packets, err := ParseLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	return packets
}

func line(dir, op, hex string) string {
	return "time=2026-09-29T00:00:00Z level=INFO msg=" + dir + " op=" + op + " size=9 hex=" + hex + "\n"
}

func run(t *testing.T, java, goLog string, options Options) (string, bool) {
	t.Helper()
	var out bytes.Buffer
	found := Compare(&out, parse(t, java), parse(t, goLog), options)
	return out.String(), found
}

func TestParseBothFormats(t *testing.T) {
	packets := parse(t, "junk\n"+line("client", "CM_MOVE", "0102")+"23:46:33.659 server SM_KEY 7 decc1481\nserver SM_X 3 \n")
	if len(packets) != 3 || !packets[0].Client || packets[0].Op != "CM_MOVE" || packets[1].Op != "SM_KEY" || len(packets[1].Body) != 4 || packets[2].Op != "SM_X" {
		t.Fatalf("parsed %+v", packets)
	}
}

func TestIdenticalLogsHaveNoDifferences(t *testing.T) {
	log := line("client", "CM_SHOW_DIALOG", "01000000") + line("server", "SM_DIALOG_WINDOW", "0100000002000000")
	out, found := run(t, log, log, Options{})
	if found || !strings.Contains(out, "no server packet differs") {
		t.Fatalf("found=%v\n%s", found, out)
	}
}

func TestOnlyInOneSideAndByteDifference(t *testing.T) {
	java := line("client", "CM_SHOW_DIALOG", "01") +
		line("server", "SM_DIALOG_WINDOW", "0100000002000000") +
		line("server", "SM_QUEST_ACCEPTED", "0a000000")
	goLog := line("client", "CM_SHOW_DIALOG", "01") +
		line("server", "SM_DIALOG_WINDOW", "0100000003000000") +
		line("server", "SM_SYSTEM_MESSAGE", "13000000")
	out, found := run(t, java, goLog, Options{})
	if !found {
		t.Fatal("expected differences")
	}
	for _, want := range []string{
		"== CM_SHOW_DIALOG",
		"only Java: SM_QUEST_ACCEPTED",
		"only Go:   SM_SYSTEM_MESSAGE",
		"differs:   SM_DIALOG_WINDOW",
		"@4..4 java=02 go=03 [dialogId]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestOrderDifference(t *testing.T) {
	java := line("client", "CM_LEVEL_READY", "") + line("server", "SM_A", "01") + line("server", "SM_B", "02")
	goLog := line("client", "CM_LEVEL_READY", "") + line("server", "SM_B", "02") + line("server", "SM_A", "01")
	out, found := run(t, java, goLog, Options{})
	if !found || !strings.Contains(out, "order differs") {
		t.Fatalf("found=%v\n%s", found, out)
	}
}

func TestVolatileFieldsAreMasked(t *testing.T) {
	// Object ids and coordinates differ between runs: masked as id/time by default, coordinates on request.
	java := line("client", "CM_MOVE", "00") + line("server", "SM_NPC_INFO", "0000803f0000003f0000c03f"+"93040100"+"c3340300")
	goLog := line("client", "CM_MOVE", "00") + line("server", "SM_NPC_INFO", "0000803f0000003f0000c03f"+"c6040000"+"c3340300")
	if out, found := run(t, java, goLog, Options{Norm: []string{"id"}}); found {
		t.Fatalf("object id should be masked:\n%s", out)
	}
	if _, found := run(t, java, goLog, Options{Norm: []string{"time"}}); !found {
		t.Fatal("object id differs and is not masked")
	}
	goCoord := strings.Replace(goLog, "0000803f", "0000803e", 1)
	if _, found := run(t, java, goCoord, Options{Norm: []string{"id"}}); !found {
		t.Fatal("coordinates should differ without the coord mask")
	}
	if out, found := run(t, java, goCoord, Options{Norm: []string{"id", "coord"}}); found {
		t.Fatalf("coordinates should be masked:\n%s", out)
	}
}

func TestLearnedObjectIdIsMaskedInOtherPackets(t *testing.T) {
	spawn := func(id string) string {
		return line("server", "SM_NPC_INFO", "000000000000000000000000"+id+"00000000")
	}
	java := line("client", "CM_MOVE", "") + spawn("93040100") + line("server", "SM_ATTACK_STATUS", "b505010093040100")
	goLog := line("client", "CM_MOVE", "") + spawn("c6040000") + line("server", "SM_ATTACK_STATUS", "b5050100c6040000")
	if out, found := run(t, java, goLog, Options{Norm: []string{"id"}}); found {
		t.Fatalf("id inside another packet should be masked:\n%s", out)
	}
}

func TestFilters(t *testing.T) {
	java := line("client", "CM_MOVE", "00") + line("server", "SM_MOVE", "01") +
		line("client", "CM_SHOW_DIALOG", "00") + line("server", "SM_DIALOG_WINDOW", "01") + line("server", "SM_MOVE", "01")
	goLog := line("client", "CM_MOVE", "00") + line("server", "SM_MOVE", "02") +
		line("client", "CM_SHOW_DIALOG", "00") + line("server", "SM_DIALOG_WINDOW", "02") + line("server", "SM_MOVE", "02")
	out, _ := run(t, java, goLog, Options{From: "CM_SHOW_DIALOG"})
	if strings.Contains(out, "== CM_MOVE") || !strings.Contains(out, "SM_DIALOG_WINDOW") {
		t.Fatalf("-from should start at the dialog:\n%s", out)
	}
	out, _ = run(t, java, goLog, Options{Skip: []string{"SM_MOVE"}})
	if strings.Contains(out, "SM_MOVE #") || !strings.Contains(out, "SM_DIALOG_WINDOW") {
		t.Fatalf("-skip should drop SM_MOVE:\n%s", out)
	}
	out, _ = run(t, java, goLog, Options{Only: []string{"SM_MOVE"}})
	if strings.Contains(out, "SM_DIALOG_WINDOW") {
		t.Fatalf("-ops should keep only SM_MOVE:\n%s", out)
	}
}

func TestRequestOnlyOnOneSide(t *testing.T) {
	java := line("client", "CM_A", "") + line("client", "CM_B", "") + line("server", "SM_X", "01")
	goLog := line("client", "CM_A", "")
	out, found := run(t, java, goLog, Options{})
	if !found || !strings.Contains(out, "request only in Java") || !strings.Contains(out, "only Java: SM_X") {
		t.Fatalf("found=%v\n%s", found, out)
	}
}

func TestIdIsNotMaskedBeforeItIsLearned(t *testing.T) {
	// The same 4 bytes are data in an early packet and an object id later: only the later one is masked.
	early := func(v string) string { return line("server", "SM_SIEGE_LOCATION_INFO", v) }
	java := line("client", "CM_A", "") + early("c6040000")
	goLog := line("client", "CM_A", "") + early("93040100")
	if _, found := run(t, java+line("server", "SM_NPC_INFO", "000000000000000000000000c604000000000000"), goLog+line("server", "SM_NPC_INFO", "00000000000000000000000093040100"+"00000000"), Options{Norm: []string{"id"}}); !found {
		t.Fatal("data before the spawn must not be masked")
	}
}

func TestLabelWindowsALogBetweenMarks(t *testing.T) {
	log := "msg=mark label=a\nmsg=client op=CM_X size=3 hex=\nmsg=server op=SM_A size=4 hex=01\n" +
		"msg=mark label=b\nmsg=client op=CM_X size=3 hex=\nmsg=server op=SM_A size=4 hex=02\n"
	path := filepath.Join(t.TempDir(), "sniff.log")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _ := ReadLog(path, "a")
	b, _ := ReadLog(path, "b")
	all, _ := ReadLog(path, "")
	if len(a) != 2 || len(b) != 2 || len(all) != 4 || a[1].Body[0] != 1 || b[1].Body[0] != 2 {
		t.Fatalf("a=%v b=%v all=%d", a, b, len(all))
	}
}
