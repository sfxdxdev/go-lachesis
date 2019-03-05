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

	needBoostrap bool
	gossipJobs   count64
	rpcJobs      count64
}

// NewNode create a new node struct
func NewNode(conf *Config,
	id uint64,
	key *ecdsa.PrivateKey,
	participants *peers.Peers,
	store poset.Store,
	trans peer.SyncPeer,
	proxy proxy.AppProxy,
	localAddr string) *Node {

	commitCh := make(chan poset.Block, 400)
	core := NewCore(id, key, participants, store, commitCh, conf.Logger)

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
	}

	signal.Notify(node.signalTERMch, syscall.SIGTERM, os.Kill)

	node.logger.WithField("participants", participants).Debug("participants")
	node.logger.WithField("pubKey", pubKey).Debug("pubKey")

	node.needBoostrap = store.NeedBootstrap()

	// Initialize
	node.setState(Gossiping)

	return &node
}

// Init initializes all the node processes
func (n *Node) Init() error {
	if n.needBoostrap {
		n.logger.Debug("Bootstrap")
		if err := n.core.Bootstrap(); err != nil {
			return err
		}
	}

	return n.core.SetHeadAndHeight()
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
		case CatchingUp:
			// TODO:
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
		if err := n.core.poset.Store.Close(); err != nil {
			n.logger.WithError(err).Debug("node::Shutdown::n.core.poset.Store.Close()")
		}
	}
}

// ID shows the ID of the node
func (n *Node) ID() uint64 {
	return n.id
}

////// SYNC //////

func (n *Node) gossip(peer *peers.Peer) error {
	// Get data from peer
	events, heights, err := n.getUnknownEventsFromPeer(peer)
	if err != nil {
		return err
	}

	// If we don't have any new events from peer -> return without error.
	if len(*events) == 0 {
		return nil
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

func (n *Node) getUnknownEventsFromPeer(peer *peers.Peer) (*[]poset.WireEvent, *map[uint64]int64, error) {
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

func (n *Node) addIntoPoset(peer *peers.Peer, events *[]poset.WireEvent) error {
	err := n.core.AddIntoPoset(peer, events)
	if err != nil {
		return err
	}

	// TODO: Run consensus

	return nil
}

func (n *Node) collectUnknownEventsForPeer(peer *peers.Peer, knownEvents *map[uint64]int64) (*[]poset.WireEvent, error) {
	// Get unknown events for source node
	n.coreLock.Lock()
	eventDiff, err := n.core.EventDiff(*knownEvents)
	n.coreLock.Unlock()

	if err != nil {
		return nil, err
	}

	// If we don't have any new events for peer -> return without error.
	if len(eventDiff) == 0 {
		return &[]poset.WireEvent{}, nil
	}

	// Convert to WireEvents
	wireEvents := n.core.ToWire(eventDiff)

	return &wireEvents, nil
}
