package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

func (s *Server) adminSpawn(p *player, args []string) {
	id, err := strconv.ParseInt(firstOf(args), 10, 32)
	if err != nil || len(args) < 1 || len(args) > 2 || len(args) == 2 && !strings.EqualFold(args[1], "norespawn") {
		s.adminUsage(p, "spawn <template_id> [norespawn]")
		return
	}
	group := &data.SpawnGroup{Map: p.WorldID, NpcID: int32(id), Pool: 1, Interval: 60, Spots: []data.Spot{{X: p.X, Y: p.Y, Z: p.Z, Heading: p.Heading}}}
	o := &object{id: s.ids.nextID(), worldID: p.WorldID, instance: p.instance, x: p.X, y: p.Y, z: p.Z, homeX: p.X, homeY: p.Y, homeZ: p.Z, heading: byte(p.Heading), interval: 60, spawnGroup: group, spawnSpot: group.Spots[0], noRespawn: len(args) == 2}
	if isGatherable(int32(id)) {
		o.gatherable = s.data.Gatherables[int32(id)]
		o.watchers = map[int32]*player{}
	} else {
		o.npc = s.data.Npcs[int32(id)]
	}
	if o.npc == nil && o.gatherable == nil {
		s.ids.release(o.id)
		s.tell(p, "There is no template with that ID.")
		return
	}
	if o.npc != nil {
		s.initNpc(o)
	}
	if s.adminSpawnGroups == nil {
		s.adminSpawnGroups = map[*data.SpawnGroup]bool{}
	}
	s.adminSpawnGroups[group] = true
	s.data.Spawns[p.WorldID] = append(s.data.Spawns[p.WorldID], group)
	s.byID[o.id] = o
	s.addObject(o)
	if o.ai != nil {
		o.ai.handleEvent(evRespawned)
	}
	s.tell(p, fmt.Sprintf("Template %d spawned (object %d).", id, o.id))
}
func (s *Server) deleteAdminObject(o *object) {
	o.noRespawn = true
	o.restore.cancel()
	o.decay.cancel()
	o.respawn.cancel()
	o.useTask.cancel()
	if o.ai != nil {
		o.ai.task.cancel()
		o.ai.talkTask.cancel()
	}
	for _, t := range o.timers {
		t.cancel()
	}
	s.removeObject(o)
	delete(s.byID, o.id)
	s.ids.release(o.id)
}
func (s *Server) adminDeleteSpawn(p *player, args []string) {
	o := s.byID[p.targetID]
	if o == nil || o.npc == nil || o.owner != nil {
		s.tell(p, "Select a spawned NPC first.")
		return
	}
	if group := o.spawnGroup; group != nil {
		for index, spot := range group.Spots {
			if spot == o.spawnSpot {
				group.Spots = slices.Delete(group.Spots, index, index+1)
				group.Pool = min(group.Pool, int32(len(group.Spots)))
				break
			}
		}
		if len(group.Spots) == 0 {
			s.data.Spawns[group.Map] = slices.DeleteFunc(s.data.Spawns[group.Map], func(candidate *data.SpawnGroup) bool { return candidate == group })
			delete(s.adminSpawnGroups, group)
		}
	}
	s.deleteAdminObject(o)
	s.tell(p, "Spawn removed.")
}
func (s *Server) adminReloadSpawns(p *player, args []string) {
	if len(args) != 0 {
		s.adminUsage(p, "reload_spawn")
		return
	}
	for _, o := range s.byID {
		if o.owner == nil && o.rift == nil && o.kisk == nil {
			s.deleteAdminObject(o)
		}
	}
	s.spawnAll()
	for key := range s.instances {
		s.spawnMap(key[0], key[1])
	}
	for _, q := range s.spawned {
		s.updateKnown(q)
	}
	s.tell(p, "Spawns reloaded.")
}
func adminDataRoot() string {
	if path := os.Getenv("AION_DATA"); path != "" {
		return path
	}
	return "/data/static_data"
}
func (s *Server) adminSaveSpawns(p *player, args []string) {
	if len(args) > 1 || len(args) == 1 && args[0] != "all" {
		s.adminUsage(p, "save_spawn [all]")
		return
	}
	dir := filepath.Join(adminDataRoot(), "spawns", "new")
	if s.adminError(p, os.MkdirAll(dir, 0755)) {
		return
	}
	for world, groups := range s.data.Spawns {
		selected := []*data.SpawnGroup{}
		for _, g := range groups {
			if len(args) == 1 || s.adminSpawnGroups[g] {
				selected = append(selected, g)
			}
		}
		if len(selected) == 0 {
			continue
		}
		document := struct {
			XMLName xml.Name           `xml:"spawns"`
			Groups  []*data.SpawnGroup `xml:"spawn"`
		}{Groups: selected}
		content, err := xml.MarshalIndent(document, "", "  ")
		if s.adminError(p, err) {
			return
		}
		path := filepath.Join(dir, fmt.Sprintf("%d.xml", world))
		if s.adminError(p, os.WriteFile(path, append([]byte(xml.Header), content...), 0644)) {
			return
		}
	}
	s.tell(p, "Spawn data saved into static_data/spawns/new.")
}
func (s *Server) adminReload(p *player, args []string) {
	if len(args) != 1 || !slices.Contains([]string{"quest", "skill", "portal", "spawn"}, args[0]) {
		s.adminUsage(p, "reload <quest|skill|portal|spawn>")
		return
	}
	loaded, err := data.Load(adminDataRoot())
	if s.adminError(p, err) {
		return
	}
	switch args[0] {
	case "quest":
		s.data.Quests = loaded.Quests
		s.data.QuestScripts = loaded.QuestScripts
		s.data.QuestStarts = loaded.QuestStarts
		s.data.QuestEnds = loaded.QuestEnds
		s.data.QuestActions = loaded.QuestActions
		s.data.QuestKills = loaded.QuestKills
		s.data.QuestDropsByNPC = loaded.QuestDropsByNPC
		s.data.QuestXMLTalks = loaded.QuestXMLTalks
		s.data.QuestCustomTalks = loaded.QuestCustomTalks
		s.data.QuestItemUses = loaded.QuestItemUses
	case "skill":
		s.data.Skills = loaded.Skills
		s.data.NpcSkills = loaded.NpcSkills
		s.fxTemplates = map[*data.SkillTemplate][]*effectTemplate{}
	case "portal":
		s.data.Portals = loaded.Portals
		s.data.PortalList = loaded.PortalList
	case "spawn":
		s.data.Spawns = loaded.Spawns
		s.adminSpawnGroups = nil
	}
	s.tell(p, args[0]+" data reloaded successfully.")
}
func (s *Server) adminBookmark(p *player, args []string) {
	if len(args) == 0 || len(args) > 2 {
		s.adminUsage(p, "bk <add|del|tele|list> [name]")
		return
	}
	list, err := s.adminDB.Bookmarks(p.ID)
	if s.adminError(p, err) {
		return
	}
	if args[0] == "list" && len(args) == 1 {
		for _, b := range list {
			s.tell(p, fmt.Sprintf("%s: map %d %.3f %.3f %.3f", b.Name, b.WorldID, b.X, b.Y, b.Z))
		}
		return
	}
	if len(args) != 2 || !slices.Contains([]string{"add", "del", "tele"}, args[0]) {
		s.adminUsage(p, "bk <add|del|tele|list> [name]")
		return
	}
	name := strings.ToLower(args[1])
	index := slices.IndexFunc(list, func(b store.Bookmark) bool { return b.Name == name })
	switch args[0] {
	case "add":
		if index >= 0 {
			s.tell(p, "Bookmark already exists.")
			return
		}
		if s.adminError(p, s.adminDB.AddBookmark(p.ID, store.Bookmark{Name: name, WorldID: p.WorldID, X: p.X, Y: p.Y, Z: p.Z})) {
			return
		}
	case "del":
		if index < 0 {
			s.tell(p, "Bookmark not found.")
			return
		}
		if s.adminError(p, s.adminDB.DeleteBookmark(p.ID, name)) {
			return
		}
	case "tele":
		if index < 0 {
			s.tell(p, "Bookmark not found.")
			return
		}
		b := list[index]
		s.teleportTo(p, b.WorldID, b.X, b.Y, b.Z, 0, 0)
	}
	s.tell(p, "Bookmark operation completed.")
}
func (s *Server) adminDrop(p *player, args []string) {
	if len(args) != 5 {
		s.adminUsage(p, "adddrop <mobid> <itemid> <min> <max> <chance>")
		return
	}
	var values [5]int64
	for i, arg := range args {
		n, err := strconv.ParseInt(arg, 10, 32)
		if err != nil {
			s.tell(p, "Only integer numbers are allowed.")
			return
		}
		values[i] = n
	}
	npc, item := int32(values[0]), int32(values[1])
	if s.data.Npcs[npc] == nil || s.data.Items[item] == nil || values[2] < 1 || values[3] < values[2] || values[4] < 0 || values[4] > 100 {
		s.tell(p, "Invalid drop template or range.")
		return
	}
	d := store.Drop{ItemID: item, Min: int32(values[2]), Max: int32(values[3]), Chance: float32(values[4])}
	if s.adminError(p, s.adminDB.AddDrop(npc, d)) {
		return
	}
	if s.drops == nil {
		s.drops = map[int32][]store.Drop{}
	}
	s.drops[npc] = append(s.drops[npc], d)
	s.tell(p, "Drop added.")
}
func (s *Server) adminAnnouncements(p *player, args []string) {
	usage := "announcements list | add <ELYOS|ASMODIANS|ALL> <NORMAL|ANNOUNCE|ORANGE|YELLOW|SHOUT> <seconds> <message> | delete <id>"
	if len(args) == 0 {
		s.adminUsage(p, usage)
		return
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			s.adminUsage(p, usage)
			return
		}
		list, err := s.adminDB.AutoAnnouncements()
		if s.adminError(p, err) {
			return
		}
		for _, a := range list {
			s.tell(p, fmt.Sprintf("%d | %s | %s | %d | %s", a.ID, a.Faction, a.Type, a.Delay, a.Text))
		}
		return
	case "add":
		if len(args) < 5 {
			s.adminUsage(p, usage)
			return
		}
		faction, kind := strings.ToUpper(args[1]), strings.ToUpper(args[2])
		delay, err := strconv.ParseInt(args[3], 10, 32)
		if err != nil || delay < 1 || !slices.Contains([]string{"ELYOS", "ASMODIANS", "ALL"}, faction) || !slices.Contains([]string{"NORMAL", "ANNOUNCE", "ORANGE", "YELLOW", "SHOUT"}, kind) {
			s.tell(p, "Invalid announcement faction, type or delay.")
			return
		}
		if s.adminError(p, s.adminDB.SaveAnnouncement(store.AutoAnnouncement{Text: strings.Join(args[4:], " "), Faction: faction, Type: kind, Delay: int32(delay)})) {
			return
		}
	case "delete":
		if len(args) != 2 {
			s.adminUsage(p, usage)
			return
		}
		id, err := strconv.ParseInt(args[1], 10, 32)
		if err != nil || id < 1 {
			s.tell(p, "Invalid announcement ID.")
			return
		}
		if s.adminError(p, s.adminDB.DeleteAnnouncement(int32(id))) {
			return
		}
	default:
		s.adminUsage(p, usage)
		return
	}
	list, err := s.adminDB.AutoAnnouncements()
	if s.adminError(p, err) {
		return
	}
	for _, t := range s.announcementTasks {
		t.cancel()
	}
	s.announcementTasks = s.startAnnouncements(list)
	s.tell(p, "Announcements updated.")
}
