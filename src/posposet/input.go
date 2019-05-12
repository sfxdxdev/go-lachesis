package posposet

import (
	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
)

type EventSource interface {
	GetEvent(hash.Event) *inter.Event
}

/*
 * Poset's methods:
 */

// GetEvent returns event.
func (p *Poset) GetEvent(h hash.Event) *Event {
	e := p.input.GetEvent(h)
	if e == nil {
		panic("got unsaved event")
	}
	return &Event{
		Event: *e,
	}
}
