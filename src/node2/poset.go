package node2

import (
	"crypto/ecdsa"

	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

type Poset interface {
	GetLastEvent(string) (*poset.EventHash, bool, error)
	GetParticipantEvents(string, int64) (*poset.EventHashes, error)
	GetEventBlock(poset.EventHash) (*poset.Event, error)
	Close() error
	GetPendingLoadedEvents() int64
	InsertEvent(poset.Event, bool) error
	SetWireInfoAndSign(*poset.Event, *ecdsa.PrivateKey) error
	GetLastBlockIndex() int64
	GetBlock(int64) (poset.Block, error)
	NewEvent([][]byte, []poset.InternalTransaction, []poset.BlockSignature, poset.EventHashes, []byte, int64, poset.FlagTable) poset.Event
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

func (p *PosetWrapper) Close() error {
	return p.poset.Store.Close()
}

func (p *PosetWrapper) GetPendingLoadedEvents() int64 {
	return p.poset.GetPendingLoadedEvents()
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

func (p *PosetWrapper) NewEvent(transactions [][]byte, internalTransactions []poset.InternalTransaction, blockSignatures []poset.BlockSignature, parents poset.EventHashes, creator []byte, index int64, ft poset.FlagTable) poset.Event {
	return poset.NewEvent(transactions, internalTransactions, blockSignatures, parents, creator, index, ft);
}