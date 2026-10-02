package main

import (
	_ "embed"
	"html/template"
	"net/http"

	"aionlightning/commands"
)

//go:embed help.html
var helpHTML string
var helpPage = template.Must(template.New("help").Parse(helpHTML))

type helpView struct {
	User          *user
	Player, Admin []commands.Command
}

func commandHelp(u *user) helpView {
	v := helpView{User: u, Player: commands.Player}
	if u.Admin() {
		v.Admin = commands.Admin
	}
	return v
}

func (p *panel) help(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := helpPage.Execute(w, commandHelp(p.current(r))); err != nil {
		p.log.Error("rendering command help", "err", err)
	}
}
