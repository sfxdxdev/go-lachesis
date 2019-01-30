package peers

import (
	"bytes"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/crypto"
)

var (
	// PeerNIL is used for nil peer id
	PeerNIL = common.Address{}
)

type Peer struct {
	ID      common.Address
	NetAddr string
	PubKey  []byte
	Used    uint64
}

// NewPeer creates a new peer based on public key and network address
func NewPeer(pubKey []byte, netAddr string) *Peer {
	peer := &Peer{
		PubKey:  pubKey,
		NetAddr: netAddr,
		Used:    0,
	}
	peer.computeID()

	return peer
}

// Equals checks peers for equality
func (p *Peer) Equals(cmp *Peer) bool {
	return p.ID == cmp.ID &&
		p.NetAddr == cmp.NetAddr &&
		bytes.Equal(p.PubKey, cmp.PubKey)
}

func (p *Peer) computeID() {
	p.ID = crypto.AddressOfPK(p.PubKey)
}
