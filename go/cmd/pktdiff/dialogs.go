package main

import (
	"fmt"
	"io"
	"strings"
)

// Verdicts of a step: the Java effects and the Go effects of the same client action.
const (
	verdictSame    = "same"
	verdictDiffers = "differs"
	verdictMissing = "missing" // Java answered with something Go did not (or Go never performed the action)
	verdictExtra   = "extra"   // Go answered with something Java did not (or Java never performed the action)
)

// verdict compares the effects of one step. Effects are compared as ordered lists of rendered packets.
func verdict(java, goServer []string) string {
	if strings.Join(java, "\n") == strings.Join(goServer, "\n") {
		return verdictSame
	}
	onlyJava, onlyGo := multisetMinus(java, goServer), multisetMinus(goServer, java)
	switch {
	case len(onlyJava) > 0 && len(onlyGo) == 0:
		return verdictMissing
	case len(onlyGo) > 0 && len(onlyJava) == 0:
		return verdictExtra
	}
	return verdictDiffers // different packets, or the same ones in another order
}

func multisetMinus(a, b []string) []string {
	count := map[string]int{}
	for _, x := range b {
		count[x]++
	}
	var out []string
	for _, x := range a {
		if count[x] > 0 {
			count[x]--
		} else {
			out = append(out, x)
		}
	}
	return out
}

// Dialogs prints the client-action timeline of both logs, one line per step with a verdict, the differing steps
// side by side, and the first divergence. It reports whether any step differs.
func Dialogs(w io.Writer, java, goLog []Packet, options Options) bool {
	java = window(java, options.From, options.Until)
	goLog = window(goLog, options.From, options.Until)
	topts := TimelineOptions{Only: options.Only, Skip: options.Skip}
	ja, ga := pruneKills(Timeline(java, topts)), pruneKills(Timeline(goLog, topts))
	keys := func(steps []Step) []string {
		out := make([]string, len(steps))
		for i, s := range steps {
			out[i] = s.Key
		}
		return out
	}
	fmt.Fprintf(w, "== client-action timeline: java %d steps, go %d steps\n", len(ja), len(ga))
	counts := map[string]int{}
	first := ""
	for n, pair := range alignNames(keys(ja), keys(ga)) {
		var jstep, gstep *Step
		if pair[0] >= 0 {
			jstep = &ja[pair[0]]
		}
		if pair[1] >= 0 {
			gstep = &ga[pair[1]]
		}
		var action, v string
		var jeff, geff []string
		switch {
		case gstep == nil:
			action, v, jeff = jstep.Action, verdictMissing, jstep.Effects
		case jstep == nil:
			action, v, geff = gstep.Action, verdictExtra, gstep.Effects
		default:
			action, v, jeff, geff = jstep.Action, verdict(jstep.Effects, gstep.Effects), jstep.Effects, gstep.Effects
		}
		counts[v]++
		note := ""
		switch {
		case gstep == nil:
			note = "   (Go never did this action)"
		case jstep == nil:
			note = "   (Java never did this action)"
		}
		fmt.Fprintf(w, "%3d %-8s %s%s\n", n+1, v, action, note)
		if v == verdictSame {
			if len(jeff) > 0 {
				fmt.Fprintf(w, "             %s\n", strings.Join(jeff, " ; "))
			}
			continue
		}
		if first == "" {
			first = fmt.Sprintf("step %d (%s) %s\n    java: %s\n    go:   %s", n+1, v, action, listOrNone(jeff), listOrNone(geff))
		}
		sideBySide(w, jeff, geff)
	}
	fmt.Fprintf(w, "-- steps: %d same, %d differs, %d missing in Go, %d extra in Go\n",
		counts[verdictSame], counts[verdictDiffers], counts[verdictMissing], counts[verdictExtra])
	if first == "" {
		fmt.Fprintln(w, "-- no divergence: every client action got the same server effects")
		return false
	}
	fmt.Fprintf(w, "-- first divergence: %s\n", first)
	return true
}

// pruneKills drops the deaths that had no quest effect: every mob the character fought is a kill step otherwise.
func pruneKills(steps []Step) []Step {
	out := steps[:0:0]
	for _, s := range steps {
		if s.Kind != "kill" || len(s.Effects) > 0 {
			out = append(out, s)
		}
	}
	return out
}

func listOrNone(effects []string) string {
	if len(effects) == 0 {
		return "(nothing)"
	}
	return strings.Join(effects, " ; ")
}

// sideBySide prints two effect lists in columns: Java left, Go right.
func sideBySide(w io.Writer, java, goServer []string) {
	width := len("java")
	for _, e := range java {
		width = max(width, len(e))
	}
	width = min(width, 56)
	fmt.Fprintf(w, "        %-*s | %s\n", width, "java", "go")
	for i := 0; i < len(java) || i < len(goServer); i++ {
		l, r := "", ""
		if i < len(java) {
			l = java[i]
		}
		if i < len(goServer) {
			r = goServer[i]
		}
		mark := " "
		if l != r {
			mark = "!"
		}
		fmt.Fprintf(w, "      %s %-*s | %s\n", mark, width, l, r)
	}
}
