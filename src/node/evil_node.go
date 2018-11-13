package node

import (
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/andrecronje/lachesis/src/crypto"
	"github.com/andrecronje/lachesis/src/net"
	"github.com/andrecronje/lachesis/src/peers"
	"github.com/andrecronje/lachesis/src/poset"
)

type EvilNode struct {
	nodeState

	conf   *Config
	logger *logrus.Entry

	id int64

	localAddr string

	peerSelector PeerSelector
	selectorLock sync.Mutex

	trans net.Transport
	netCh <-chan net.RPC

	commitCh chan poset.Block

	shutdownCh chan struct{}

	controlTimer *ControlTimer

	start        time.Time
	syncRequests int
	syncErrors   int

	needBoostrap bool
}

func NewEvilNode(
	conf *Config,
	id int64,
	key *ecdsa.PrivateKey,
	participants *peers.Peers,
	trans net.Transport) *EvilNode {

	localAddr := trans.LocalAddr()

	commitCh := make(chan poset.Block, 400)

	pubKey := fmt.Sprintf("0x%X", crypto.FromECDSAPub(&key.PublicKey))

	peerSelector := NewRandomPeerSelector(participants, pubKey)

	node := EvilNode{
		id:           id,
		conf:         conf,
		localAddr:    localAddr,
		logger:       conf.Logger.WithField("this_id", id),
		peerSelector: peerSelector,
		trans:        trans,
		netCh:        trans.Consumer(),
		commitCh:     commitCh,
		shutdownCh:   make(chan struct{}),
		controlTimer: NewRandomControlTimer(),
		start:        time.Now(),
	}

	node.logger.WithField("peers", participants).Debug("pmap")
	node.logger.WithField("pubKey", pubKey).Debug("pubKey")

	// Initialize
	node.setState(Gossiping)

	return &node
}

func (n *EvilNode) Init() error {
	var peerAddresses []string
	for _, p := range n.peerSelector.Peers().ToPeerSlice() {
		peerAddresses = append(peerAddresses, p.NetAddr)
	}
	n.logger.WithField("peers", peerAddresses).Debug("Initialize Node")
	return nil
}

func (n *EvilNode) RunAsync(gossip bool) {
	n.logger.Debug("RunAsync(gossip bool)")
	go n.Run(gossip)
}

func (n *EvilNode) Run(gossip bool) {
	// The ControlTimer allows the background routines to control the
	// heartbeat timer when the node is in the Gossiping state. The timer should
	// only be running when there are uncommitted transactions in the system.
	go n.controlTimer.Run(n.conf.HeartbeatTimeout)

	// Execute some background work regardless of the state of the node.
	// Process SumbitTx and CommitBlock requests
	go n.doBackgroundWork()

	// TODO: remove Sleep() from node.go also
	// make pause before gossiping test transactions to allow all nodes come up
	//time.Sleep(time.Duration(n.conf.TestDelay) * time.Second)

	// Execute Node State Machine
	for {
		// Run different routines depending on node state
		state := n.getState()
		n.logger.WithField("state", state.String()).Debug("RunAsync(gossip bool)")

		switch state {
		case Gossiping:
			n.lachesis(gossip)
		case CatchingUp:
			n.fastForward()
		case Shutdown:
			return
		}
	}
}

func (n *EvilNode) resetTimer() {
	if !n.controlTimer.set {
		ts := n.conf.HeartbeatTimeout
		//Slow gossip if nothing interesting to say
		// TODO: set ts if nothing to say only
		ts = time.Duration(time.Second)
		n.controlTimer.resetCh <- ts
	}
}

func (n *EvilNode) doBackgroundWork() {
	for {
		select {
		case <-n.shutdownCh:
			return
		}
	}
}

// lachesis is interrupted when a gossip function, launched asynchronously, changes
// the state from Gossiping to CatchingUp, or when the node is shutdown.
// Otherwise, it processes RPC requests, periodicaly initiates gossip while there
// is something to gossip about, or waits.
func (n *EvilNode) lachesis(gossip bool) {
	returnCh := make(chan struct{}, 100)
	for {
		select {
		case rpc := <-n.netCh:
			n.goFunc(func() {
				n.logger.Debug("Processing RPC")
				n.processRPC(rpc)
				n.resetTimer()
			})
		case <-n.controlTimer.tickCh:
			if gossip {
				n.logger.Debug("Gossip")
				peer := n.peerSelector.Next()
				n.goFunc(func() { n.gossip(peer.NetAddr, returnCh) })
			}
			n.resetTimer()
		case <-returnCh:
			return
		case <-n.shutdownCh:
			return
		}
	}
}

func (n *EvilNode) processRPC(rpc net.RPC) {
	switch cmd := rpc.Command.(type) {
	case *net.SyncRequest:
		n.processSyncRequest(rpc, cmd)
	case *net.EagerSyncRequest:
		n.processEagerSyncRequest(rpc, cmd)
	case *net.FastForwardRequest:
		n.processFastForwardRequest(rpc, cmd)
	default:
		n.logger.WithField("cmd", rpc.Command).Error("Unexpected RPC command")
		rpc.Respond(nil, fmt.Errorf("unexpected command"))
	}
}

func (n *EvilNode) processSyncRequest(rpc net.RPC, cmd *net.SyncRequest) {
	n.logger.WithFields(logrus.Fields{
		"from_id": cmd.FromID,
		"known":   cmd.Known,
	}).Debug("processSyncRequest(rpc net.RPC, cmd *net.SyncRequest)")

	resp := &net.SyncResponse{
		FromID: n.id,
	}
	var respErr error

	// TODO: make fake Diff
	wireEvents := make([]poset.WireEvent, 0, 0)
	resp.Events = wireEvents

	// make fake Known
	knownEvents := make(map[int64]int64)
	resp.Known = knownEvents

	n.logger.WithFields(logrus.Fields{
		"events": len(resp.Events),
		"known":  resp.Known,
		"error":  respErr,
	}).Debug("SyncRequest Received")

	rpc.Respond(resp, respErr)
}

func (n *EvilNode) processEagerSyncRequest(rpc net.RPC, cmd *net.EagerSyncRequest) {
	n.logger.WithFields(logrus.Fields{
		"from_id": cmd.FromID,
		"events":  len(cmd.Events),
	}).Debug("processEagerSyncRequest(rpc net.RPC, cmd *net.EagerSyncRequest)")

	// TODO: fake sync with cmd.Events
	success := true

	resp := &net.EagerSyncResponse{
		FromID:  n.id,
		Success: success,
	}
	rpc.Respond(resp, nil)
}

func (n *EvilNode) processFastForwardRequest(rpc net.RPC, cmd *net.FastForwardRequest) {
	n.logger.WithFields(logrus.Fields{
		"from": cmd.FromID,
	}).Debug("processFastForwardRequest(rpc net.RPC, cmd *net.FastForwardRequest)")

	resp := &net.FastForwardResponse{
		FromID: n.id,
	}
	var respErr error

	// TODO: make fake latest Frame (core.GetAnchorBlockWithFrame())
	block, frame := poset.Block{}, poset.Frame{}
	resp.Block = block
	resp.Frame = frame

	// TODO: make fake snapshot (proxy.GetSnapshot(block.Index()))
	snapshot := []byte{}
	resp.Snapshot = snapshot

	n.logger.WithFields(logrus.Fields{
		"Events": len(resp.Frame.Events),
		"Error":  respErr,
	}).Debug("FastForwardRequest Received")
	rpc.Respond(resp, respErr)
}

// This function is usually called in a go-routine and needs to inform the
// calling routine (usually the lachesis routine) when it is time to exit the
// Gossiping state and return.
func (n *EvilNode) gossip(peerAddr string, parentReturnCh chan struct{}) error {

	// pull
	syncLimit, otherKnownEvents, err := n.pull(peerAddr)
	if err != nil {
		return err
	}

	// check and handle syncLimit
	if syncLimit {
		n.logger.WithField("from", peerAddr).Debug("SyncLimit")
		n.setState(CatchingUp)
		parentReturnCh <- struct{}{}
		return nil
	}

	// push
	err = n.push(peerAddr, otherKnownEvents)
	if err != nil {
		return err
	}

	// update peer selector
	n.selectorLock.Lock()
	n.peerSelector.UpdateLast(peerAddr)
	n.selectorLock.Unlock()

	return nil
}

func (n *EvilNode) pull(peerAddr string) (syncLimit bool, otherKnownEvents map[int64]int64, err error) {

	// make fake Known
	knownEvents := make(map[int64]int64)
	if len(knownEvents) < 1 {
		return
	}

	// Send SyncRequest
	start := time.Now()
	resp, err := n.requestSync(peerAddr, knownEvents)
	elapsed := time.Since(start)
	n.logger.WithField("Duration", elapsed.Nanoseconds()).Debug("n.requestSync(peerAddr, knownEvents)")
	// FIXIT: should we catch io.EOF error here and how we process it?
	//	if err == io.EOF {
	//		return false, nil, nil
	//	}
	if err != nil {
		n.logger.WithField("Error", err).Error("n.requestSync(peerAddr, knownEvents)")
		return false, nil, err
	}
	n.logger.WithFields(logrus.Fields{
		"from_id":     resp.FromID,
		"sync_limit":  resp.SyncLimit,
		"events":      len(resp.Events),
		"known":       resp.Known,
		"knownEvents": knownEvents,
	}).Debug("SyncResponse")

	if resp.SyncLimit {
		return true, nil, nil
	}

	// Add Events to poset and create new Head if necessary
	err = n.sync(resp.Events)
	if err != nil {
		n.logger.WithField("error", err).Error("n.sync(resp.Events)")
		return false, nil, err
	}

	return false, resp.Known, nil
}

func (n *EvilNode) push(peerAddr string, knownEvents map[int64]int64) error {

	// TODO: make fake Diff (again!?)
	if false {
		// Convert to WireEvents
		wireEvents := make([]poset.WireEvent, 0, 0)

		// Create and Send EagerSyncRequest
		start := time.Now()
		n.logger.WithField("wireEvents", wireEvents).Debug("Sending n.requestEagerSync.wireEvents")
		resp2, err := n.requestEagerSync(peerAddr, wireEvents)
		elapsed := time.Since(start)
		n.logger.WithField("Duration", elapsed.Nanoseconds()).Debug("n.requestEagerSync(peerAddr, wireEvents)")
		if err != nil {
			n.logger.WithField("Error", err).Error("n.requestEagerSync(peerAddr, wireEvents)")
			return err
		}
		n.logger.WithFields(logrus.Fields{
			"from_id": resp2.FromID,
			"success": resp2.Success,
		}).Debug("EagerSyncResponse")
	}

	return nil
}

func (n *EvilNode) fastForward() error {
	n.logger.Debug("fastForward()")

	// wait until sync routines finish
	n.waitRoutines()

	// fastForwardRequest
	peer := n.peerSelector.Next()
	start := time.Now()
	resp, err := n.requestFastForward(peer.NetAddr)
	elapsed := time.Since(start)
	n.logger.WithField("Duration", elapsed.Nanoseconds()).Debug("n.requestFastForward(peer.NetAddr)")
	if err != nil {
		n.logger.WithField("Error", err).Error("n.requestFastForward(peer.NetAddr)")
		return err
	}
	n.logger.WithFields(logrus.Fields{
		"from_id":              resp.FromID,
		"block_index":          resp.Block.Index(),
		"block_round_received": resp.Block.RoundReceived(),
		"frame_events":         len(resp.Frame.Events),
		"frame_roots":          resp.Frame.Roots,
		"snapshot":             resp.Snapshot,
	}).Debug("FastForwardResponse")

	// prepare core. ie: fresh poset

	// err = n.core.FastForward(peer.PubKeyHex, resp.Block, resp.Frame)

	// TODO: update app from snapshot?
	/*
		err = n.proxy.Restore(resp.Snapshot)
		if err != nil {
			n.logger.WithField("Error", err).Error("n.proxy.Restore(resp.Snapshot)")
			return err
		}
	*/
	n.setState(Gossiping)

	return nil
}

func (n *EvilNode) requestSync(target string, known map[int64]int64) (net.SyncResponse, error) {

	args := net.SyncRequest{
		FromID: n.id,
		Known:  known,
	}

	var out net.SyncResponse
	err := n.trans.Sync(target, &args, &out)
	//n.logger.WithField("out", out).Debug("requestSync(target string, known map[int64]int64)")
	return out, err
}

func (n *EvilNode) requestEagerSync(target string, events []poset.WireEvent) (net.EagerSyncResponse, error) {
	args := net.EagerSyncRequest{
		FromID: n.id,
		Events: events,
	}

	var out net.EagerSyncResponse
	n.logger.WithFields(logrus.Fields{
		"target": target,
	}).Debug("requestEagerSync(target string, events []poset.WireEvent)")
	err := n.trans.EagerSync(target, &args, &out)

	return out, err
}

func (n *EvilNode) requestFastForward(target string) (net.FastForwardResponse, error) {
	n.logger.WithFields(logrus.Fields{
		"target": target,
	}).Debug("requestFastForward(target string) (net.FastForwardResponse, error)")

	args := net.FastForwardRequest{
		FromID: n.id,
	}

	var out net.FastForwardResponse
	err := n.trans.FastForward(target, &args, &out)

	return out, err
}

func (n *EvilNode) sync(events []poset.WireEvent) error {
	/*
		// Insert Events in Poset and create new Head if necessary
		start := time.Now()
		err := n.core.Sync(events)
		elapsed := time.Since(start)
		n.logger.WithField("Duration", elapsed.Nanoseconds()).Debug("n.core.Sync(events)")
		if err != nil {
			return err
		}

		// Run consensus methods
		start = time.Now()
		err = n.core.RunConsensus()
		elapsed = time.Since(start)
		n.logger.WithField("Duration", elapsed.Nanoseconds()).Debug("n.core.RunConsensus()")
		if err != nil {
			return err
		}
	*/

	return nil
}

func (n *EvilNode) commit(block poset.Block) error {
	/*
		stateHash := []byte{0, 1, 2}
		_, err := n.proxy.CommitBlock(block)
		if err != nil {
			n.logger.WithError(err).Debug("commit(block poset.Block)")
		}

		n.logger.WithFields(logrus.Fields{
			"block":      block.Index(),
			"state_hash": fmt.Sprintf("%X", stateHash),
			// "err":        err,
		}).Debug("commit(eventBlock poset.EventBlock)")

		// XXX what do we do in case of error. Retry? This has to do with the
		// Lachesis <-> App interface. Think about it.

		// An error here could be that the endpoint is not configured, not all
		// nodes will be sending blocks to clients, in these cases -no_client can be
		// used, alternatively should check for the error here and handle it
		// appropriately

		// There is no point in using the stateHash if we know it is wrong
		// if err == nil {
		if true {
			// inmem statehash would be different than proxy statehash
			// inmem is simply the hash of transactions
			// this requires a 1:1 relationship with nodes and clients
			// multiple nodes can't read from the same client

			block.Body.StateHash = stateHash
			n.coreLock.Lock()
			defer n.coreLock.Unlock()
			sig, err := n.core.SignBlock(block)
			if err != nil {
				return err
			}
			n.core.AddBlockSignature(sig)
		}
	*/
	return nil
}

/*
func (n *EvilNode) addTransaction(tx []byte) {
	n.coreLock.Lock()
	defer n.coreLock.Unlock()
	n.core.AddTransactions([][]byte{tx})
}

func (n *EvilNode) addInternalTransaction(tx poset.InternalTransaction) {
	n.coreLock.Lock()
	defer n.coreLock.Unlock()
	n.core.AddInternalTransactions([]poset.InternalTransaction{tx})
}
*/
func (n *EvilNode) Shutdown() {
	if n.getState() != Shutdown {
		// n.mqtt.FireEvent("Shutdown()", "/mq/lachesis/node")
		n.logger.Debug("Shutdown()")

		// Exit any non-shutdown state immediately
		n.setState(Shutdown)

		// Stop and wait for concurrent operations
		close(n.shutdownCh)
		n.waitRoutines()

		// For some reason this needs to be called after closing the shutdownCh
		// Not entirely sure why...
		n.controlTimer.Shutdown()

		// transport should only be closed once all concurrent operations
		// are finished otherwise they will panic trying to use close objects
		n.trans.Close()
	}
}

func (n *EvilNode) SyncRate() float64 {
	var syncErrorRate float64
	if n.syncRequests != 0 {
		syncErrorRate = float64(n.syncErrors) / float64(n.syncRequests)
	}
	return 1 - syncErrorRate
}

/*
func (n *EvilNode) GetParticipants() (*peers.Peers, error) {
	return n.core.poset.Store.Participants()
}

func (n *EvilNode) GetEvent(event string) (poset.Event, error) {
	return n.core.poset.Store.GetEvent(event)
}

func (n *EvilNode) GetLastEventFrom(participant string) (string, bool, error) {
	return n.core.poset.Store.LastEventFrom(participant)
}

func (n *EvilNode) GetKnownEvents() map[int64]int64 {
	return n.core.poset.Store.KnownEvents()
}

func (n *EvilNode) GetEvents() (map[int64]int64, error) {
	res := n.core.KnownEvents()
	return res, nil
}

func (n *EvilNode) GetConsensusEvents() []string {
	return n.core.poset.Store.ConsensusEvents()
}

func (n *EvilNode) GetConsensusTransactionsCount() uint64 {
	return n.core.GetConsensusTransactionsCount()
}

func (n *EvilNode) GetRound(roundIndex int64) (poset.RoundInfo, error) {
	return n.core.poset.Store.GetRound(roundIndex)
}

func (n *EvilNode) GetLastRound() int64 {
	return n.core.poset.Store.LastRound()
}

func (n *EvilNode) GetRoundWitnesses(roundIndex int64) []string {
	return n.core.poset.Store.RoundWitnesses(roundIndex)
}

func (n *EvilNode) GetRoundEvents(roundIndex int64) int {
	return n.core.poset.Store.RoundEvents(roundIndex)
}

func (n *EvilNode) GetRoot(rootIndex string) (poset.Root, error) {
	return n.core.poset.Store.GetRoot(rootIndex)
}

func (n *EvilNode) GetBlock(blockIndex int64) (poset.Block, error) {
	return n.core.poset.Store.GetBlock(blockIndex)
}
*/
func (n *EvilNode) ID() int64 {
	return n.id
}
