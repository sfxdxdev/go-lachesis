// +build !debug

package poset

import (
	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
	"github.com/Fantom-foundation/go-lachesis/src/state"
)

// Store provides an interface for persistent and non-persistent stores
// to store key lachesis consensus information on a node.
type Store interface {
	TopologicalEvents() ([]Event, error) // returns event in topological order
	CacheSize() int
	Participants() (*peers.Peers, error)
	RepertoireByID() map[common.Address]*peers.Peer
	RootsBySelfParent() (map[EventHash]Root, error)
	GetEventBlock(EventHash) (Event, error)
	SetEvent(Event) error
	ParticipantEvents(common.Address, int64) (EventHashes, error)
	ParticipantEvent(common.Address, int64) (EventHash, error)
	LastEventFrom(common.Address) (EventHash, bool, error)
	LastConsensusEventFrom(common.Address) (EventHash, bool, error)
	KnownEvents() map[common.Address]int64
	ConsensusEvents() EventHashes
	ConsensusEventsCount() int64
	AddConsensusEvent(Event) error
	GetRoundCreated(int64) (RoundCreated, error)
	SetRoundCreated(int64, RoundCreated) error
	GetRoundReceived(int64) (RoundReceived, error)
	SetRoundReceived(int64, RoundReceived) error
	LastRound() int64
	RoundClothos(int64) EventHashes
	RoundEvents(int64) int
	GetRoot(common.Address) (Root, error)
	GetBlock(int64) (Block, error)
	SetBlock(Block) error
	LastBlockIndex() int64
	GetFrame(int64) (Frame, error)
	SetFrame(Frame) error
	Reset(map[common.Address]Root) error
	Close() error
	NeedBoostrap() bool // Was the store loaded from existing db
	StorePath() string
	// StateDB returns state database
	StateDB() state.Database
	StateRoot() common.Hash
}
