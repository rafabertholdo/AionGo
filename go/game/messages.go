package game

import (
	"fmt"

	"aionlightning/wire"
)

// System message ids (SystemMessageId, SM_SYSTEM_MESSAGE).
const (
	msgDeleteCharacterInLegion = 1300306
	msgChannelEntered          = 1390122 // "You have entered channel %0."
)

// descriptionID is a system message parameter that names a client string by its id.
type descriptionID int32

// systemMessage is SM_SYSTEM_MESSAGE: a client string by id, with parameters.
func systemMessage(code int32, params ...any) *wire.Writer {
	w := wire.Packet(smSystemMessage)
	w.H(0x13)
	w.D(0)
	w.D(code)
	w.C(byte(len(params)))
	for _, p := range params {
		if id, ok := p.(descriptionID); ok {
			w.H(0x24)
			w.D(int32(id))
			w.H(0)
		} else {
			w.S(fmt.Sprint(p))
		}
	}
	w.C(0)
	return w
}
