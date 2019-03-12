package node2

import (
	"crypto/ecdsa"

	"github.com/Fantom-foundation/go-lachesis/src/peers"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

type Poset interface {
	GetLastEvent(string) (*poset.EventHash, bool, error)
	GetParticipantEvents(string, int64) (*poset.EventHashes, error)
	GetEventBlock(poset.EventHash) (*poset.Event, error)
	GetRoot(string) (*poset.Root, error)
	Close() error
	SetCore(core *Core)
	GetPendingLoadedEvents() int64
	GetParticipants() (*peers.Peers, error)
	Bootstrap() error
	ReadWireInfo(poset.WireEvent) (*poset.Event, error)
	InsertEvent(poset.Event, bool) error
	SetWireInfoAndSign(*poset.Event, *ecdsa.PrivateKey) error
	GetLastBlockIndex() int64
	GetBlock(int64) (poset.Block, error)
}

type PosetWrapper struct {
	poset *poset.Poset
}

func NewPosetWrapper(poset *poset.Poset) *PosetWrapper {
	return &PosetWrapper{
		poset: poset,
	}
}

func (p *PosetWrapper) GetLastEvent(pubKey string) (*poset.EventHash, bool, error) {
	eventHash, isRoot, err := p.poset.Store.LastEventFrom(pubKey)
	if err != nil {
		return nil, false, err
	}

	return &eventHash, isRoot, nil
}

func (p *PosetWrapper) GetParticipantEvents(participant string, skip int64) (*poset.EventHashes, error) {
	events, err := p.poset.Store.ParticipantEvents(participant, skip)
	if err != nil {
		return nil, err
	}

	return &events, nil
}

func (p *PosetWrapper) GetEventBlock(hash poset.EventHash) (*poset.Event, error) {
	event, err := p.poset.Store.GetEventBlock(hash)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

func (p *PosetWrapper) GetRoot(hex string) (*poset.Root, error) {
	root, err := p.poset.Store.GetRoot(hex)
	if err != nil {
		return nil, err
	}

	return &root, nil
}

func (p *PosetWrapper) Close() error {
	return p.poset.Store.Close()
}

func (p *PosetWrapper) SetCore(core *Core) {
	p.poset.SetCore(core)
}

func (p *PosetWrapper) GetPendingLoadedEvents() int64 {
	return p.poset.GetPendingLoadedEvents()
}

func (p *PosetWrapper) GetParticipants() (*peers.Peers, error) {
	participants, err := p.poset.Store.Participants()
	if err != nil {
		return nil, err
	}

	return participants, nil
}

func (p *PosetWrapper) Bootstrap() error {
	return p.poset.Bootstrap()
}

func (p *PosetWrapper) ReadWireInfo(wireEvent poset.WireEvent) (*poset.Event, error) {
	ev, err := p.poset.ReadWireInfo(wireEvent)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func (p *PosetWrapper) InsertEvent(event poset.Event, setWireInfo bool) error {
	return p.poset.InsertEvent(event, setWireInfo)
}

func (p *PosetWrapper) SetWireInfoAndSign(event *poset.Event, privKey *ecdsa.PrivateKey) error {
	return p.poset.SetWireInfoAndSign(event, privKey)
}

func (p *PosetWrapper) GetLastBlockIndex() int64 {
	return p.poset.Store.LastBlockIndex()
}

func (p *PosetWrapper) GetBlock(target int64) (poset.Block, error) {
	return p.poset.Store.GetBlock(target)
}
