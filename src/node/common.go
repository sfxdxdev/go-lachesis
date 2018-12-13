package node

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	_ "testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/crypto"
	"github.com/Fantom-foundation/go-lachesis/src/dummy"
	"github.com/Fantom-foundation/go-lachesis/src/net"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

const delay = 10 * time.Millisecond

// NodeList is a list of connected nodes for tests purposes
type NodeList map[*ecdsa.PrivateKey]*Node

// NewNodeList makes, fills and runs NodeList instance
func NewNodeList(count int, logger *logrus.Logger) NodeList {
	nodes := make(NodeList, count)
	participants := peers.NewPeers()
	config := DefaultConfig()
	config.Logger = logger

	for i := 0; i < count; i++ {
		addr, transp := net.NewInmemTransport("")
		key, _ := crypto.GenerateECDSAKey()
		pubKey := fmt.Sprintf("0x%X", crypto.FromECDSAPub(&key.PublicKey))
		peer := peers.NewPeer(pubKey, addr)
		n := NewNode(
			config,
			peer.ID,
			key,
			participants,
			poset.NewInmemStore(participants, config.CacheSize),
			transp,
			dummy.NewInmemDummyApp(logger))

		participants.AddPeer(peer)
		nodes[key] = n
	}

	for _, n := range nodes {
		n.Init()
		n.RunAsync(true)
	}

	return nodes
}

// Keys returns the all PrivateKeys slice
func (n NodeList) Keys() []*ecdsa.PrivateKey {
	keys := make([]*ecdsa.PrivateKey, len(n))
	i := 0
	for key, _ := range n {
		keys[i] = key
		i++
	}
	return keys
}

// Values returns the all nodes slice
func (n NodeList) Values() []*Node {
	nodes := make([]*Node, len(n))
	i := 0
	for _, node := range n {
		nodes[i] = node
		i++
	}
	return nodes
}

// StartRandTxStream sends random txs to nodes until stop() called
func (n NodeList) StartRandTxStream() (stop func()) {
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
				keys := n.Keys()
				count := len(n)
				for i := 0; i < count; i++ {
					j := rand.Intn(count)
					node := n[keys[j]]
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
func (n NodeList) WaitForBlock(target int64) {
	n.waitForAll(func(node *Node) bool {
		if target > node.GetLastBlockIndex() {
			return false
		}
		block, _ := node.GetBlock(target)
		return len(block.GetStateHash()) != 0
	})
}

// WaitForConsensusTxsCount waits until the target block has retrieved a state hash from the app
func (n NodeList) WaitForConsensusTxsCount(target uint64) {
	n.waitForAll(func(node *Node) bool {
		return node.GetConsensusTransactionsCount() >= target
	})
}

// Close shutdowns all nodes
func (n NodeList) Close() {
	for _, node := range n {
		node.Shutdown()
	}
}

// waitForAll waits until completed() returns true for all nodes
func (n NodeList) waitForAll(completed func(*Node) bool) {
waiting:
	for {
		time.Sleep(delay)
		for _, node := range n {
			if !completed(node) {
				continue waiting
			}
		}
		return
	}
}
