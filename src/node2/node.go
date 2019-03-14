package node2

import (
	"crypto/ecdsa"
	"github.com/go-errors/errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/node2/wire"
	"github.com/Fantom-foundation/go-lachesis/src/peer"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
	"github.com/Fantom-foundation/go-lachesis/src/proxy"
)

type Node struct {
	*nodeState

	conf   *Config
	logger *logrus.Entry

	id       uint64
	core     *Core
	coreLock sync.Mutex

	trans peer.SyncPeer
	proxy proxy.AppProxy

	submitCh         chan []byte
	submitInternalCh chan poset.InternalTransaction
	commitCh         chan poset.Block
	shutdownCh       chan struct{}
	signalTERMch     chan os.Signal

	controlTimer *ControlTimer

	start        time.Time
	syncRequests int
	syncErrors   int

	needBootstrap bool
	gossipJobs    count64
	rpcJobs       count64

	testMode bool
}

// NewNode create a new node struct
func NewNode(conf *Config,
	id uint64,
	key *ecdsa.PrivateKey,
	participants *peers.Peers,
	pst Poset,
	commitCh chan poset.Block,
	needBootstrap bool,
	trans peer.SyncPeer,
	proxy proxy.AppProxy,
	localAddr string) *Node {

	core := NewCore(id, key, participants, pst, conf.Logger, conf.MaxEventsPayloadSize)

	pubKey := core.HexID()

	node := Node{
		id:               id,
		conf:             conf,
		core:             core,
		logger:           conf.Logger.WithField("this_id", id),
		trans:            trans,
		proxy:            proxy,
		submitCh:         proxy.SubmitCh(),
		submitInternalCh: proxy.SubmitInternalCh(),
		commitCh:         commitCh,
		shutdownCh:       make(chan struct{}),
		controlTimer:     NewRandomControlTimer(),
		start:            time.Now(),
		gossipJobs:       0,
		rpcJobs:          0,
		nodeState:        newNodeState(),
		signalTERMch:     make(chan os.Signal, 1),
		testMode:         false,
	}

	signal.Notify(node.signalTERMch, syscall.SIGTERM, os.Kill)

	node.logger.WithField("participants", participants).Debug("participants")
	node.logger.WithField("pubKey", pubKey).Debug("pubKey")

	node.needBootstrap = needBootstrap

	// Initialize
	node.setState(Gossiping)

	return &node
}

// Init initializes all the node processes
func (n *Node) Init() error {
	if n.needBootstrap {
		n.logger.Debug("Bootstrap")
		if err := n.core.Bootstrap(); err != nil {
			return err
		}
	}

	return n.core.SetHeadAndHeight()
}

// CommitCh return channel to write commit messages.
// Maybe better way to call n.commit in poset
// through node interface ???
func (n *Node) CommitCh() chan<- poset.Block {
	return n.commitCh
}

// SubmitCh func for test
func (n *Node) SubmitCh(tx []byte) error {
	n.proxy.SubmitCh() <- []byte(tx)
	return nil
}

// Run core run loop, takes care of all processes
func (n *Node) Run() {
	// The ControlTimer allows the background routines to control the
	// heartbeat timer when the node is in the Gossiping state. The timer should
	// only be running when there are uncommitted transactions in the system.
	go n.controlTimer.Run(n.conf.HeartbeatTimeout)

	// Process SubmitTx
	go n.txHandler()

	// Execute Node State Machine
	for {
		// Run different routines depending on node state
		state := n.getState()

		switch state {
		case Gossiping:
			n.requestHandler()
		case Stop:
			// do nothing in Stop state
		case Shutdown:
			return
		}
	}
}

// Stop stops the node from gossiping
func (n *Node) Stop() {
	n.setState(Stop)
}

// Shutdown the node
func (n *Node) Shutdown() {
	if n.getState() != Shutdown {
		n.logger.Debug("Shutdown()")

		// Exit any non-shutdown state immediately
		n.setState(Shutdown)

		// Stop and wait for concurrent operations
		close(n.shutdownCh)
		n.waitRoutines()

		// For some reason this needs to be called after closing the shutdownCh
		// Not entirely sure why...
		n.controlTimer.Shutdown()

		// transport and store should only be closed once all concurrent operations
		// are finished otherwise they will panic trying to use close objects
		n.trans.Close()
		if err := n.core.poset.Close(); err != nil {
			n.logger.WithError(err).Debug("node::Shutdown::n.core.poset.Close()")
		}
	}
}

// ID shows the ID of the node
func (n *Node) ID() uint64 {
	return n.id
}

// GetTransactionPoolCount returns the count of all pending transactions
func (n *Node) GetTransactionPoolCount() int64 {
	return n.core.GetTransactionPoolCount()
}

// HeartbeatTimeout returns heartbeat timeout.
func (n *Node) HeartbeatTimeout() time.Duration {
	return n.conf.HeartbeatTimeout
}

// KnownEvents return known events.
func (n *Node) KnownEvents() map[uint64]int64 {
	return n.core.GetKnownHeights()
}

// StartTime returns node start time.
func (n *Node) StartTime() time.Time {
	return n.start
}

// State returns current state.
func (n *Node) State() string {
	return n.state.String()
}

// SyncLimit returns sync limit from config.
func (n *Node) SyncLimit() int64 {
	return n.conf.SyncLimit
}

// SyncRate returns the current synchronization (talking to over nodes) rate in ms
func (n *Node) SyncRate() float64 {
	var syncErrorRate float64
	if n.syncRequests != 0 {
		syncErrorRate = float64(n.syncErrors) / float64(n.syncRequests)
	}
	return 1 - syncErrorRate
}

////// SYNC //////

func (n *Node) gossip(peer *peers.Peer) error {

	// Get data from peer
	events, heights, err := n.getUnknownEventsFromPeer(peer)
	if err != nil {
		return err
	}

	// Add data into poset & run consensus
	err = n.addIntoPoset(peer, events)
	if err != nil {
		return err
	}

	// Collect data for peer
	events, err = n.collectUnknownEventsForPeer(peer, heights)
	if err != nil {
		return err
	}

	// If we don't have any new events for peer -> return without error.
	if len(*events) == 0 {
		return nil
	}

	// Send data to peer
	resp, err := n.requestWithEvents(peer.NetAddr, *events)
	if err != nil {
		return err
	}

	if !resp.Success {
		return errors.New("Error during requestWithEvents process")
	}

	return nil
}

func (n *Node) getUnknownEventsFromPeer(peer *peers.Peer) (*[]wire.Event, *map[uint64]int64, error) {
	// Get known heights for all known nodes.
	n.coreLock.Lock()
	knownHeights := n.core.GetKnownHeights()
	n.coreLock.Unlock()

	// Send info to peer & get unknown events
	resp, err := n.requestWithHeights(peer.NetAddr, knownHeights)
	if err != nil {
		return nil, nil, err
	}

	return &resp.Events, &resp.Known, nil
}

func (n *Node) addIntoPoset(peer *peers.Peer, events *[]wire.Event) error {
	err := n.core.AddIntoPoset(peer, events)
	if err != nil {
		return err
	}

	// TODO: Run consensus

	return nil
}

func (n *Node) collectUnknownEventsForPeer(peer *peers.Peer, knownEvents *map[uint64]int64) (*[]wire.Event, error) {
	// Get unknown events for source node
	n.coreLock.Lock()
	eventDiff, err := n.core.EventDiff(*knownEvents)
	n.coreLock.Unlock()

	if err != nil {
		return nil, err
	}

	// If we don't have any new events for peer -> return without error.
	if len(eventDiff) == 0 {
		return &[]wire.Event{}, nil
	}

	// Convert to WireEvents
	wireEvents := n.core.ToWire(eventDiff)

	return &wireEvents, nil
}
