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

	peerSelector PeerSelector

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
	selectorInitFunc SelectorCreationFn,
	selectorInitArgs SelectorCreationFnArgs,
	localAddr string) *Node {

	commitCh := make(chan poset.Block, 400)
	core := NewCore(id, key, participants, store, commitCh, conf.Logger)

	pubKey := core.HexID()

	if args, ok := selectorInitArgs.(SmartPeerSelectorCreationFnArgs); ok {
		args.GetFlagTable = core.poset.GetPeerFlagTableOfRandomUndeterminedEvent
		args.LocalAddr = localAddr
		selectorInitArgs = args
	}

	peerSelector := selectorInitFunc(participants, selectorInitArgs)

	node := Node{
		id:               id,
		conf:             conf,
		core:             core,
		logger:           conf.Logger.WithField("this_id", id),
		peerSelector:     peerSelector,
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
	var peerAddresses []string
	for _, p := range n.peerSelector.Peers().ToPeerSlice() {
		peerAddresses = append(peerAddresses, p.NetAddr)
	}
	n.logger.WithField("peers", peerAddresses).Debug("Initialize Node")

	if n.needBoostrap {
		n.logger.Debug("Bootstrap")
		if err := n.core.Bootstrap(); err != nil {
			return err
		}
	}

	return n.core.SetHeadAndHeight()
}

////// SYNC //////

// TODO: Run gossip for each peer using goroutines
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

	// TODO: Are we still need it?
	// update peer selector
	n.peerSelector.UpdateLast(peer.NetAddr)

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
