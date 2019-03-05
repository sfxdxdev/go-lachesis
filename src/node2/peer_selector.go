package node2

import (
	"math"
	"math/rand"

	"github.com/Fantom-foundation/go-lachesis/src/peers"
)

// PeerSelector provides an interface for the lachesis node to
// update the last peer it gossiped with and select the next peer
// to gossip with
type PeerSelector interface {
	Peers() *peers.Peers
	UpdateLast(peer string)
	Next() *peers.Peer
}

// RandomPeerSelector is a randomized peer selection struct
type RandomPeerSelector struct {
	peers     *peers.Peers
	localAddr string
	last      string
}

// SelectorCreationFnArgs specifies the union of possible arguments that can be extracted to create a variant of PeerSelector
type SelectorCreationFnArgs interface{}

// SelectorCreationFn declares the function signature to create variants of PeerSelector
type SelectorCreationFn func(*peers.Peers, interface{}) PeerSelector

// RandomPeerSelectorCreationFnArgs arguments for RandomPeerSelector
type RandomPeerSelectorCreationFnArgs struct {
	LocalAddr string
}

// GetFlagTableFn declares flag table function signature
type GetFlagTableFn func() (map[string]int64, error)

// SmartPeerSelector provides selection based on FlagTable of a randomly chosen undermined event
type SmartPeerSelector struct {
	peers        *peers.Peers
	localAddr    string
	last         string
	GetFlagTable GetFlagTableFn
}

// SmartPeerSelectorCreationFnArgs specifies which additional arguments are required to create a SmartPeerSelector
type SmartPeerSelectorCreationFnArgs struct {
	GetFlagTable GetFlagTableFn
	LocalAddr    string
}

// NewRandomPeerSelector creates a new random peer selector
func NewRandomPeerSelector(participants *peers.Peers, args RandomPeerSelectorCreationFnArgs) *RandomPeerSelector {
	return &RandomPeerSelector{
		localAddr: args.LocalAddr,
		peers:     participants,
	}
}

// NewRandomPeerSelectorWrapper implements SelectorCreationFn to allow dynamic creation of RandomPeerSelector ie NewNode
func NewRandomPeerSelectorWrapper(participants *peers.Peers, args interface{}) PeerSelector {
	return NewRandomPeerSelector(participants, args.(RandomPeerSelectorCreationFnArgs))
}

// Peers returns all known peers
func (ps *RandomPeerSelector) Peers() *peers.Peers {
	return ps.peers
}

// UpdateLast sets the last peer communicated with (to avoid double talk)
func (ps *RandomPeerSelector) UpdateLast(peer string) {
	ps.last = peer
}

// Next returns the next randomly selected peer(s) to communicate with
func (ps *RandomPeerSelector) Next() *peers.Peer {
	slice := ps.peers.ToPeerSlice()
	selectablePeers := peers.ExcludePeers(slice, ps.localAddr, ps.last)

	if len(selectablePeers) < 1 {
		selectablePeers = slice
	}

	i := rand.Intn(len(selectablePeers))

	peer := selectablePeers[i]

	return peer
}

// NewSmartPeerSelector creates a new smart peer selection struct
func NewSmartPeerSelector(participants *peers.Peers, args SmartPeerSelectorCreationFnArgs) *SmartPeerSelector {

	return &SmartPeerSelector{
		localAddr:    args.LocalAddr,
		peers:        participants,
		GetFlagTable: args.GetFlagTable,
	}
}

// NewSmartPeerSelectorWrapper implements SelectorCreationFn to allow dynamic creation of SmartPeerSelector ie NewNode
func NewSmartPeerSelectorWrapper(participants *peers.Peers, args interface{}) PeerSelector {
	return NewSmartPeerSelector(participants, args.(SmartPeerSelectorCreationFnArgs))
}

// Peers returns all known peers
func (ps *SmartPeerSelector) Peers() *peers.Peers {
	return ps.peers
}

// UpdateLast sets the last peer communicated with (avoid double talk)
func (ps *SmartPeerSelector) UpdateLast(peer string) {
	// We need an exclusive access to ps.last for writing;
	// let use peers' lock instead of adding additional lock.
	// ps.last is accessed for read under peers' lock
	ps.peers.Lock()
	defer ps.peers.Unlock()

	ps.last = peer
}

// Next returns the next peer based on the flag table cost function selection
func (ps *SmartPeerSelector) Next() *peers.Peer {
	flagTable, err := ps.GetFlagTable()
	if err != nil {
		flagTable = nil
	}

	ps.peers.Lock()
	defer ps.peers.Unlock()

	sortedSrc := ps.peers.ToPeerByUsedSlice()
	n := int(2*len(sortedSrc)/3 + 1)
	if n < len(sortedSrc) {
		sortedSrc = sortedSrc[0:n]
	}
	selected := make([]*peers.Peer, len(sortedSrc))
	sCount := 0
	flagged := make([]*peers.Peer, len(sortedSrc))
	fCount := 0
	minUsedIdx := 0
	minUsedVal := int64(math.MaxInt64)
	var lastused []*peers.Peer

	for _, p := range sortedSrc {
		if p.NetAddr == ps.localAddr {
			continue
		}
		if p.NetAddr == ps.last || p.PubKeyHex == ps.last {
			lastused = append(lastused, p)
			continue
		}

		if f, ok := flagTable[p.PubKeyHex]; ok && f == 1 {
			flagged[fCount] = p
			fCount += 1
			continue
		}

		if p.Used < minUsedVal {
			minUsedVal = p.Used
			minUsedIdx = sCount
		}
		selected[sCount] = p
		sCount += 1
	}

	selected = selected[minUsedIdx:sCount]
	if len(selected) < 1 {
		selected = flagged[0:fCount]
	}
	if len(selected) < 1 {
		selected = lastused
	}
	if len(selected) == 1 {
		selected[0].Used++
		return selected[0]
	}
	if len(selected) < 1 {
		return nil
	}

	i := rand.Intn(len(selected))
	selected[i].Used++
	return selected[i]
}
