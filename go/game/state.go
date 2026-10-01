package game

// stateSet is the connection states a client packet is accepted in.
type stateSet uint8

const (
	inConnected stateSet = 1 << iota
	inAuthed
	inGame
)
