package poset

import (
	"fmt"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
)

// Key struct
type Key struct {
	x EventHash
	y EventHash
}

// ToString converts key to string
func (k Key) ToString() string {
	return fmt.Sprintf("{%s, %s}", k.x, k.y)
}

// ParentRoundInfo struct
type ParentRoundInfo struct {
	round   int
	isRoot  bool
	Atropos int
}

// NewBaseParentRoundInfo constructor
func NewBaseParentRoundInfo() ParentRoundInfo {
	return ParentRoundInfo{
		round:  -1,
		isRoot: false,
	}
}

// ------------------------------------------------------------------------------

// ParticipantEventsCache struct
type ParticipantEventsCache struct {
	participants *peers.Peers
	rim          *common.RollingIndexMap
}

// NewParticipantEventsCache constructor
func NewParticipantEventsCache(size int, participants *peers.Peers) *ParticipantEventsCache {
	return &ParticipantEventsCache{
		participants: participants,
		rim:          common.NewRollingIndexMap("ParticipantEvents", size, participants.ToIDSlice()),
	}
}

// AddPeer adds peer to cache and rolling index map, returns error if it failed to add to map
func (pec *ParticipantEventsCache) AddPeer(peer *peers.Peer) error {
	pec.participants.AddPeer(peer)
	return pec.rim.AddKey(peer.ID)
}

// Get return participant events with index > skip
func (pec *ParticipantEventsCache) Get(participant common.Address, skipIndex int64) (EventHashes, error) {
	pe, err := pec.rim.Get(participant, skipIndex)
	if err != nil {
		return EventHashes{}, err
	}

	res := make(EventHashes, len(pe))
	for k := 0; k < len(pe); k++ {
		res[k].Set(pe[k].([]byte))
	}
	return res, nil
}

// GetItem get event for participant at index
func (pec *ParticipantEventsCache) GetItem(participant common.Address, index int64) (hash EventHash, err error) {
	item, err := pec.rim.GetItem(participant, index)
	if err != nil {
		return
	}

	hash.Set(item.([]byte))
	return
}

// GetLast get last event for participant
func (pec *ParticipantEventsCache) GetLast(participant common.Address) (hash EventHash, err error) {
	last, err := pec.rim.GetLast(participant)
	if err != nil {
		return
	}

	hash.Set(last.([]byte))
	return
}

// Set the event for the participant
func (pec *ParticipantEventsCache) Set(participant common.Address, hash EventHash, index int64) error {
	return pec.rim.Set(participant, hash.Bytes(), index)
}

// Known returns [participant id] => lastKnownIndex
func (pec *ParticipantEventsCache) Known() map[common.Address]int64 {
	return pec.rim.Known()
}

// Reset resets the event cache
func (pec *ParticipantEventsCache) Reset() error {
	return pec.rim.Reset()
}

// Import from another event cache
func (pec *ParticipantEventsCache) Import(other *ParticipantEventsCache) {
	pec.rim.Import(other.rim)
}

// ------------------------------------------------------------------------------

// ParticipantBlockSignaturesCache struct
type ParticipantBlockSignaturesCache struct {
	participants *peers.Peers
	rim          *common.RollingIndexMap
}

// NewParticipantBlockSignaturesCache constructor
func NewParticipantBlockSignaturesCache(size int, participants *peers.Peers) *ParticipantBlockSignaturesCache {
	return &ParticipantBlockSignaturesCache{
		participants: participants,
		rim:          common.NewRollingIndexMap("ParticipantBlockSignatures", size, participants.ToIDSlice()),
	}
}

// Get return participant BlockSignatures where index > skip
func (psc *ParticipantBlockSignaturesCache) Get(participant common.Address, skipIndex int64) ([]BlockSignature, error) {
	ps, err := psc.rim.Get(participant, skipIndex)
	if err != nil {
		return []BlockSignature{}, err
	}

	res := make([]BlockSignature, len(ps))
	for k := 0; k < len(ps); k++ {
		res[k] = ps[k].(BlockSignature)
	}
	return res, nil
}

// GetItem get block signature at index for participant
func (psc *ParticipantBlockSignaturesCache) GetItem(participant common.Address, index int64) (BlockSignature, error) {
	item, err := psc.rim.GetItem(participant, index)
	if err != nil {
		return BlockSignature{}, err
	}
	return item.(BlockSignature), nil
}

// GetLast get last block signature for participant
func (psc *ParticipantBlockSignaturesCache) GetLast(participant common.Address) (BlockSignature, error) {
	last, err := psc.rim.GetLast(participant)

	if err != nil {
		return BlockSignature{}, err
	}

	return last.(BlockSignature), nil
}

// Set sets the last block signature for the participant
func (psc *ParticipantBlockSignaturesCache) Set(participant common.Address, sig BlockSignature) error {
	return psc.rim.Set(participant, sig, sig.Index)
}

// Known returns [participant id] => last BlockSignature Index
func (psc *ParticipantBlockSignaturesCache) Known() map[common.Address]int64 {
	return psc.rim.Known()
}

// Reset resets the block signature cache
func (psc *ParticipantBlockSignaturesCache) Reset() error {
	return psc.rim.Reset()
}
