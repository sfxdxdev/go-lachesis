package posnode

import (
	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
)

// Consensus is a consensus interface.
type Consensus interface {
	Stake
	Frame
	// Consensus bringing events to the consensus.
	Consensus(event *inter.Event)
	//GetEvent(event )
}

type Frame interface {
	// GetEventFrame returns frame num of event.
	GetEventFrame(e hash.Event) *uint64
}

type Stake interface {
	// GetStakeOf returns stake of peer as fraction from one.
	GetStakeOf(hash.Peer) float64
}
