package peers

import (
	"bytes"
	"sort"
	"sync"

	"github.com/Fantom-foundation/go-lachesis/src/common"
)

// IdIndex map of peers sorted by ID
type IdIndex map[common.Address]*Peer

// Listener for listening for new peers joining
type Listener func(*Peer)

// Peers struct for all known peers for this node
type Peers struct {
	sync.RWMutex
	Sorted    []*Peer
	ByID      IdIndex
	Listeners []Listener
}

// Len returns the length of peers
func (p *Peers) Len() int {
	p.RLock()
	defer p.RUnlock()

	return len(p.Sorted)
}

/* Constructors */

// NewPeers creates a new peers struct
func NewPeers() *Peers {
	return &Peers{
		ByID: make(IdIndex),
	}
}

// NewPeersFromSlice create a new peers struct from a subset of peers
func NewPeersFromSlice(source []*Peer) *Peers {
	peers := NewPeers()

	for _, peer := range source {
		peers.addPeerRaw(peer)
	}

	peers.internalSort()

	return peers
}

/* Add Methods */

// Add a peer without sorting the set.
// Useful for adding a bunch of peers at the same time
// This method is private and is not protected by mutex.
func (p *Peers) addPeerRaw(peer *Peer) {
	if peer.ID == PeerNIL {
		peer.computeID()
	}
	p.ByID[peer.ID] = peer
}

// AddPeer adds a peer to the peers struct
func (p *Peers) AddPeer(peer *Peer) {
	p.Lock()
	p.addPeerRaw(peer)
	p.internalSort()
	p.Unlock()
	p.EmitNewPeer(peer)
}

func (p *Peers) unsortedSlice() []*Peer {
	arr := make([]*Peer, 0, len(p.ByID))
	for _, p := range p.ByID {
		arr = append(arr, p)
	}
	return arr
}

func (p *Peers) internalSort() {
	arr := p.unsortedSlice()
	sort.Sort(ByID(arr))
	p.Sorted = arr
}

/* Remove Methods */

// RemovePeer removes a peer from the peers struct
func (p *Peers) RemovePeer(peer *Peer) {
	if peer.ID == PeerNIL {
		peer.computeID()
	}

	p.Lock()
	defer p.Unlock()

	if _, ok := p.ByID[peer.ID]; !ok {
		return
	}

	delete(p.ByID, peer.ID)
	p.internalSort()
}

/* ToSlice Methods */

// ToPeerSlice returns a slice of peers sorted
func (p *Peers) ToPeerSlice() []*Peer {
	return p.Sorted
}

// ToPeerByUsedSlice sorted peers list
func (p *Peers) ToPeerByUsedSlice() []*Peer {
	arr := p.unsortedSlice()
	sort.Sort(ByUsed(arr))
	return arr
}

// ToPubKeySlice peers struct by public key.
func (p *Peers) ToPubKeySlice() [][]byte {
	p.RLock()
	defer p.RUnlock()

	res := make([][]byte, 0, len(p.Sorted))
	for _, peer := range p.Sorted {
		res = append(res, peer.PubKey)
	}

	return res
}

// ToIDSlice peers struct by ID
func (p *Peers) ToIDSlice() []common.Address {
	p.RLock()
	defer p.RUnlock()

	res := make([]common.Address, 0, len(p.Sorted))
	for _, peer := range p.Sorted {
		res = append(res, peer.ID)
	}

	return res
}

/* EventListener */

// OnNewPeer on new peer joined event trigger listener
func (p *Peers) OnNewPeer(cb func(*Peer)) {
	p.Listeners = append(p.Listeners, cb)
}

// EmitNewPeer emits an event for all listeners as soon as a peer joins
func (p *Peers) EmitNewPeer(peer *Peer) {
	for _, listener := range p.Listeners {
		listener(peer)
	}
}

/*
 * Staff
 */

// ExcludePeer is used to exclude a single peer from a list of peers.
func ExcludePeer(peers []*Peer, peer string) (int, []*Peer) {
	index := -1
	otherPeers := make([]*Peer, 0, len(peers))
	for i, p := range peers {
		if p.NetAddr != peer && common.ToHex(p.PubKey) != peer {
			otherPeers = append(otherPeers, p)
		} else {
			index = i
		}
	}
	return index, otherPeers
}

// ExcludePeers is used to exclude multiple peers from a list of peers.
func ExcludePeers(peers []*Peer, local string, last string) []*Peer {
	otherPeers := make([]*Peer, 0, len(peers))
	for _, p := range peers {
		if p.NetAddr != local &&
			p.NetAddr != last &&
			common.ToHex(p.PubKey) != local &&
			common.ToHex(p.PubKey) != last {
			otherPeers = append(otherPeers, p)
		}
	}
	return otherPeers
}

/*
 * Sorting
 */

// ByID sorted by ID peers list.
type ByID []*Peer

func (a ByID) Len() int      { return len(a) }
func (a ByID) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByID) Less(i, j int) bool {
	return bytes.Compare(a[i].ID[:], a[j].ID[:]) < 0
}

// ByUsed sorted by Used peers list.
type ByUsed []*Peer

func (a ByUsed) Len() int      { return len(a) }
func (a ByUsed) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByUsed) Less(i, j int) bool {
	return a[i].Used > a[j].Used
}
