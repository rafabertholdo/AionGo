package game

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// Petitions: AL-Game's PetitionService, CM_PETITION, SM_PETITION and //petition. A player has at most one open petition;
// the game masters online are told of a new one and answer it by mail.
// ponytail: the queue lives in memory (loaded at start) and in the petitions table; visMu guards it.

func init() { handlers[cmPetition] = (*conn).petition }

// petitionSaver keeps petitions; store.Store does.
type petitionSaver interface {
	Petitions() ([]*store.Petition, error)
	PetitionByID(int32) (*store.Petition, error)
	NextPetitionID() (int32, error)
	InsertPetition(*store.Petition) error
	DeletePetition(playerID int32) error
	SetPetitionReplied(int32) error
	PlayerName(int32) (string, error)
}

const (
	petitionCancel       = 2
	msgPetitionCancelled = 1300552 // "Petition %0 was cancelled."
	msgPetitionRemaining = 1300553
	petitionsPerPlayer   = 49 // the count SM_PETITION and the cancel message show, as AL-Game's
	adminPetitionList    = 5
)

// PetitionType element ids.
const (
	petitionStuck       = 256
	petitionRestoration = 512
	petitionBug         = 768
	petitionQuest       = 1024
	petitionBehavior    = 1280
)

// loadPetitions is PetitionService's constructor.
func (s *Server) loadPetitions() error {
	list, err := s.petitionDB.Petitions()
	s.petitions = list
	return err
}

func (s *Server) petitionOf(playerID int32) *store.Petition {
	for _, p := range s.petitions {
		if p.PlayerID == playerID {
			return p
		}
	}
	return nil
}

// petitionPacket is SM_PETITION: the player's petition and its place in the queue, or none.
func (s *Server) petitionPacket(p *store.Petition) *wire.Writer {
	w := wire.Packet(smPetition)
	if p == nil {
		w.D(0)
		w.D(0)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		return w
	}
	ahead := slices.IndexFunc(s.petitions, func(o *store.Petition) bool { return o.PlayerID == p.PlayerID })
	if ahead < 0 {
		ahead = len(s.petitions)
	}
	const perPetition, betweenPetitions = 15, 30
	w.C(1)
	w.D(100)
	w.H(uint16(ahead))
	w.S(strconv.Itoa(int(p.ID)))
	w.H(0)
	w.C(50)
	w.C(petitionsPerPlayer)
	w.H(uint16(betweenPetitions + ahead*(perPetition+betweenPetitions)))
	w.D(0)
	return w
}

// petition is CM_PETITION: register a petition, or cancel the player's own.
func (c *conn) petition(r *wire.Reader) {
	action := r.H()
	var title, text, extra string
	if action == petitionCancel {
		r.D()
	} else if parts := strings.SplitN(r.S(), "/", 3); len(parts) == 3 {
		title, text, extra = parts[0], parts[1], parts[2]
	} else {
		return
	}
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		open := s.petitionOf(p.ID)
		switch {
		case action == petitionCancel:
			// ponytail: AL-Game files an empty petition when there is none to cancel; nothing happens here.
			if open != nil {
				s.deletePetition(p.ID)
				p.conn.send(systemMessage(msgPetitionCancelled, open.ID))
				p.conn.send(systemMessage(msgPetitionRemaining, petitionsPerPlayer))
			}
		case open == nil:
			if registered := s.registerPetition(p, int32(action), title, text, extra); registered != nil {
				p.conn.send(s.petitionPacket(registered))
			}
		}
	})
}

// registerPetition is PetitionService.registerPetition: it tells the game masters online.
func (s *Server) registerPetition(p *player, kind int32, title, text, extra string) *store.Petition {
	id, err := s.petitionDB.NextPetitionID()
	petition := &store.Petition{ID: id, PlayerID: p.ID, Type: kind, Title: title, Message: text, Extra: extra}
	if err == nil {
		err = s.petitionDB.InsertPetition(petition)
	}
	if err != nil {
		s.log.Error("saving a petition", "character", p.Name, "err", err)
		return nil
	}
	s.petitions = append(s.petitions, petition)
	for _, gm := range s.spawned {
		if gm.conn.account.accessLevel > 0 {
			s.tell(gm, fmt.Sprintf("New Support Petition from: %s (#%d)", p.Name, id))
		}
	}
	return petition
}

// deletePetition is PetitionService.deletePetition: the player's petition goes, and the others move up.
func (s *Server) deletePetition(playerID int32) {
	s.petitions = slices.DeleteFunc(s.petitions, func(p *store.Petition) bool { return p.PlayerID == playerID })
	if err := s.petitionDB.DeletePetition(playerID); err != nil {
		s.log.Error("deleting a petition", "err", err)
	}
	s.rebroadcastPetitions(playerID)
}

// closePetition is PetitionService.setPetitionReplied.
func (s *Server) closePetition(petition *store.Petition) {
	s.petitions = slices.DeleteFunc(s.petitions, func(p *store.Petition) bool { return p == petition })
	if err := s.petitionDB.SetPetitionReplied(petition.ID); err != nil {
		s.log.Error("closing a petition", "err", err)
	}
	s.rebroadcastPetitions(petition.PlayerID)
}

// rebroadcastPetitions tells the owner of a petition that is gone that it is, and the others their new place.
func (s *Server) rebroadcastPetitions(gone int32) {
	if p := s.spawned[gone]; p != nil {
		p.conn.send(s.petitionPacket(nil))
	}
	for _, petition := range s.petitions {
		if p := s.spawned[petition.PlayerID]; p != nil {
			p.conn.send(s.petitionPacket(petition))
		}
	}
}

// petitionLogin is PetitionService.onPlayerLogin.
func (s *Server) petitionLogin(p *player) {
	if petition := s.petitionOf(p.ID); petition != nil {
		p.conn.send(s.petitionPacket(petition))
	}
}

// adminPetition is //petition [id [delete | reply <message>]].
func (s *Server) adminPetition(p *player, rest string) {
	params := strings.SplitN(strings.TrimSpace(rest), " ", 3)
	if params[0] == "" {
		s.tell(p, fmt.Sprintf("%d unprocessed petitions.", len(s.petitions)))
		first := s.petitions[:min(len(s.petitions), adminPetitionList)]
		s.tell(p, fmt.Sprintf("== %d first petitions to reply ==", len(first)))
		for _, petition := range first {
			s.tell(p, fmt.Sprintf("%d | %s", petition.ID, petition.Title))
		}
		return
	}
	id, err := strconv.Atoi(params[0])
	if err != nil {
		s.tell(p, "Invalid petition id.")
		return
	}
	petition, err := s.petitionDB.PetitionByID(int32(id))
	if err != nil {
		s.log.Error("reading a petition", "err", err)
	}
	if petition == nil {
		s.tell(p, fmt.Sprintf("There is no petition with id #%d", id))
		return
	}
	owner, online := "", s.spawned[petition.PlayerID]
	if online != nil {
		owner = online.Name
	} else if owner, err = s.petitionDB.PlayerName(petition.PlayerID); err != nil {
		s.log.Error("reading a name", "err", err)
	}
	switch {
	case len(params) == 1:
		state := "Offline"
		if online != nil {
			state = "Online"
		}
		s.tell(p, fmt.Sprintf("== Petition #%d ==\nPlayer: %s (%s)\nType: %s\nTitle: %s\nText: %s\n= Additional Data =\n%s",
			id, owner, state, petitionTypeName(petition.Type), petition.Title, petition.Message, petitionExtra(petition)))
	case len(params) == 2 && params[1] == "delete":
		s.deletePetition(petition.PlayerID)
		s.tell(p, fmt.Sprintf("Petition #%d deleted.", id))
	case len(params) == 3 && params[1] == "reply":
		s.sendLetter(p, owner, "GM-Re:"+petition.Title, params[2], 0, 0, 0)
		if open := s.petitionOf(petition.PlayerID); open != nil && open.ID == petition.ID {
			s.closePetition(open)
		}
		s.tell(p, fmt.Sprintf("Your reply has been sent to %s. Petition is now closed.", owner))
	}
}

// petitionTypeName is Petition.getHumanizedValue; as in AL-Game, suggestions and inquiries come out unknown.
func petitionTypeName(kind int32) string {
	switch kind {
	case petitionStuck:
		return "Character Stuck"
	case petitionRestoration:
		return "Character Restoration"
	case petitionBug:
		return "Bug"
	case petitionQuest:
		return "Quest"
	case petitionBehavior:
		return "Unacceptable Behavior"
	}
	return "Unknown"
}

// petitionExtra is Petition.getFormattedAdditionalData; a petition with too few fields shows its data as it is.
func petitionExtra(p *store.Petition) string {
	parts := strings.Split(p.Extra, "/")
	switch {
	case p.Type == petitionStuck:
		return "Character Location: " + p.Extra
	case p.Type == petitionRestoration:
		return "Category: " + p.Extra
	case p.Type == petitionBug && len(parts) >= 2:
		text := "Time Occured: " + parts[0] + "\nZone and Coords: " + parts[1]
		if len(parts) > 2 {
			text += "\nHow to Replicate: " + parts[2]
		}
		return text
	case p.Type == petitionQuest:
		return "Quest Title: " + p.Extra
	case p.Type == petitionBehavior && len(parts) >= 3:
		return "Time Occured: " + parts[0] + "\nCharacter Name: " + parts[1] + "\nCategory: " + parts[2]
	case p.Type == 1536:
		return "Category: " + p.Extra
	case p.Type == 65280:
		return "Petition Category: " + p.Extra
	}
	return p.Extra
}
