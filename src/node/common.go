package node

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	_ "testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/andrecronje/lachesis/src/crypto"
	"github.com/andrecronje/lachesis/src/dummy"
	"github.com/andrecronje/lachesis/src/net"
	"github.com/andrecronje/lachesis/src/peers"
	"github.com/andrecronje/lachesis/src/poset"
)

const delay = 100 * time.Millisecond

// NodeList is a list of connected nodes for tests purposes
type NodeList struct {
	nodes map[*ecdsa.PrivateKey]*Node
	evils map[*ecdsa.PrivateKey]*EvilNode
}

// NewNodeList makes, fills and runs NodeList instance
func NewNodeList(count int, evils int, logger *logrus.Logger) *NodeList {
	list := &NodeList{
		nodes: make(map[*ecdsa.PrivateKey]*Node, count),
		evils: make(map[*ecdsa.PrivateKey]*EvilNode, evils),
	}
	participants := peers.NewPeers()

	for i := 0; i < (count + evils); i++ {
		config := DefaultConfig()
		addr, transp := net.NewInmemTransport("")
		key, _ := crypto.GenerateECDSAKey()
		pubKey := fmt.Sprintf("0x%X", crypto.FromECDSAPub(&key.PublicKey))
		peer := peers.NewPeer(pubKey, addr)
		participants.AddPeer(peer)
		if i < count {
			list.nodes[key] = NewNode(
				config,
				peer.ID,
				key,
				participants,
				poset.NewInmemStore(participants, config.CacheSize),
				transp,
				dummy.NewInmemDummyApp(logger))
		} else {
			list.evils[key] = NewEvilNode(
				config,
				peer.ID,
				key,
				participants,
				transp)
		}
	}

	for _, n := range list.nodes {
		n.Init()
		n.RunAsync(true)
	}

	for _, e := range list.evils {
		e.Init()
		e.RunAsync(true)
	}

	return list
}

// Keys returns the all PrivateKeys slice
func (l *NodeList) Keys() []*ecdsa.PrivateKey {
	keys := make([]*ecdsa.PrivateKey, len(l.nodes))
	i := 0
	for key, _ := range l.nodes {
		keys[i] = key
		i++
	}
	return keys
}

// Values returns the all nodes slice
func (l *NodeList) Values() []*Node {
	nodes := make([]*Node, len(l.nodes))
	i := 0
	for _, node := range l.nodes {
		nodes[i] = node
		i++
	}
	return nodes
}

// StartRandTxStream sends random txs to nodes until stop() called
func (l *NodeList) StartRandTxStream() (stop func()) {
	stopCh := make(chan struct{})

	stop = func() {
		close(stopCh)
	}

	go func() {
		seq := 0
		for {
			select {
			case <-stopCh:
				return
			case <-time.After(delay):
				keys := l.Keys()
				count := len(l.nodes)
				for i := 0; i < count; i++ {
					j := rand.Intn(count)
					node := l.nodes[keys[j]]
					tx := []byte(fmt.Sprintf("node#%d transaction %d", node.ID(), seq))
					node.PushTx(tx)
					seq++
				}
			}
		}
	}()

	return
}

// WaitForBlock waits until the target block has retrieved a state hash from the app
func (l *NodeList) WaitForBlock(target int64) {
LOOP:
	for {
		time.Sleep(delay)
		for _, node := range l.nodes {
			if target > node.GetLastBlockIndex() {
				continue LOOP
			}
			block, _ := node.GetBlock(target)
			if len(block.StateHash()) == 0 {
				continue LOOP
			}
		}
		return
	}
}

// Shutdown stops nodes goroutines
func (l *NodeList) Shutdown() {
	for _, e := range l.evils {
		e.Shutdown()
	}
	for _, n := range l.nodes {
		n.Shutdown()
	}
}
