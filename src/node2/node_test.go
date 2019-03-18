package node2

import (
	"crypto/ecdsa"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/crypto"
	"github.com/Fantom-foundation/go-lachesis/src/dummy"
	"github.com/Fantom-foundation/go-lachesis/src/log"
	"github.com/Fantom-foundation/go-lachesis/src/peer"
	"github.com/Fantom-foundation/go-lachesis/src/peer/fakenet"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

type TestData struct {
	PoolSize   int
	Logger     *logrus.Logger
	Config     *Config
	BackConfig *peer.BackendConfig
	Network    *fakenet.Network
	CreateFu   peer.CreateSyncClientFunc
	Keys       []*ecdsa.PrivateKey
	Adds       []string
	PeersSlice []*peers.Peer
	Peers      *peers.Peers
}

func InitTestData(t *testing.T, peersCount int, poolSize int) *TestData {
	network, createFu := createNetwork()
	keys, p, adds := initPeers(peersCount, network)

	return &TestData{
		PoolSize:   poolSize,
		Logger:     common.NewTestLogger(t),
		Config:     TestConfig(t),
		BackConfig: peer.NewBackendConfig(),
		Network:    network,
		CreateFu:   createFu,
		Keys:       keys,
		Adds:       adds,
		PeersSlice: p.ToPeerSlice(),
		Peers:      p,
	}
}

func initPeers(
	number int, network *fakenet.Network) ([]*ecdsa.PrivateKey, *peers.Peers, []string) {

	var keys []*ecdsa.PrivateKey
	var adds []string

	ps := peers.NewPeers()

	for i := 0; i < number; i++ {
		key, _ := crypto.GenerateECDSAKey()
		keys = append(keys, key)
		addr := network.RandomAddress()
		adds = append(adds, addr)

		ps.AddPeer(peers.NewPeer(
			fmt.Sprintf("0x%X", crypto.FromECDSAPub(&keys[i].PublicKey)),
			addr,
		))
	}

	return keys, ps, adds
}

func createNetwork() (*fakenet.Network, peer.CreateSyncClientFunc) {
	network := fakenet.NewNetwork()

	createFu := func(target string,
		timeout time.Duration) (peer.SyncClient, error) {

		rpcCli, err := peer.NewRPCClient(
			peer.TCP, target, time.Second, network.CreateNetConn)
		if err != nil {
			return nil, err
		}

		return peer.NewClient(rpcCli)
	}

	return network, createFu
}

func createTransport(t *testing.T, logger logrus.FieldLogger,
	backConf *peer.BackendConfig, addr string, poolSize int,
	clientFu peer.CreateSyncClientFunc,
	listenerFu peer.CreateListenerFunc) peer.SyncPeer {

	producer := peer.NewProducer(poolSize, time.Second, clientFu)

	backend := peer.NewBackend(backConf, logger, listenerFu)
	if err := backend.ListenAndServe(peer.TCP, addr); err != nil {
		t.Fatal(err)
	}

	return peer.NewTransport(logger, producer, backend)
}

func transportClose(t *testing.T, syncPeer peer.SyncPeer) {
	if err := syncPeer.Close(); err != nil {
		t.Fatal(err)
	}
}

func createNode(t *testing.T, logger *logrus.Logger, config *Config,
	id uint64, key *ecdsa.PrivateKey, participants *peers.Peers,
	trans peer.SyncPeer, localAddr string, testMode bool) (*Node, *MockPoset) {

	db := poset.NewInmemStore(participants, config.CacheSize, nil)
	app := dummy.NewInmemDummyApp(logger)

	if logger == nil {
		logger = logrus.New()
		logger.Level = logrus.DebugLevel
		lachesis_log.NewLocal(logger, logger.Level.String())
	}

	commitCh := make(chan poset.Block, 400)

	// logEntry := logger.WithField("id", id)
	// pst := poset.NewPoset(participants, db, commitCh, logEntry)
	// posetWrapper := NewPosetWrapper(pst)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	posetWrapper := NewMockPoset(ctrl)

	node := NewNode(config, id, key, participants, posetWrapper, commitCh, db.NeedBootstrap(), trans, app, localAddr)
	node.testMode = testMode

	// Mock for init process
	posetWrapper.EXPECT().GetLastEvent(gomock.Any()).Return(&poset.EventHash{}, false, nil).Times(1)

	event := &poset.Event{
		Message: &poset.EventMessage{
			Body: &poset.EventBody{
				Index: 0,
			},
		},
	}
	posetWrapper.EXPECT().GetEventBlock(gomock.Any()).Return(event, nil).Times(1)

	if err := node.Init(); err != nil {
		t.Fatal(err)
	}

	go node.Run()

	return node, posetWrapper
}

func gossip(nodes []*Node, target int64, shutdown bool, timeout time.Duration) error {
	for _, n := range nodes {
		node := n
		go func() {
			node.Run()
		}()
	}
	err := bombardAndWait(nodes, target, timeout)
	if err != nil {
		return err
	}
	if shutdown {
		for _, n := range nodes {
			n.Shutdown()
		}
	}
	return nil
}

func bombardAndWait(nodes []*Node, target int64, timeout time.Duration) error {

	quit := make(chan struct{})
	go func() {
		seq := make(map[int]int)
		for {
			select {
			case <-quit:
				return
			default:
				n := rand.Intn(len(nodes))
				node := nodes[n]
				if err := submitTransaction(node, []byte(
					fmt.Sprintf("node%d transaction %d", n, seq[n]))); err != nil {
					panic(err)
				}
				seq[n] = seq[n] + 1
				time.Sleep(3 * time.Millisecond)
			}
		}
	}()
	tag := "beginning"

	// wait until all nodes have at least 'target' blocks
	stopper := time.After(timeout)
	for {
		select {
		case <-stopper:
			return fmt.Errorf("timeout in %v", tag)
		default:
		}
		time.Sleep(10 * time.Millisecond)
		done := true
		for _, n := range nodes {
			ce := n.core.poset.GetLastBlockIndex()
			if ce < target {
				done = false
				tag = fmt.Sprintf("ce<target:%v<%v", ce, target)
				break
			} else {
				// wait until the target block has retrieved a state hash from
				// the app
				targetBlock, _ := n.core.poset.GetBlock(target)
				if len(targetBlock.GetStateHash()) == 0 {
					done = false
					tag = "stateHash==0"
					break
				}
			}
		}
		if done {
			break
		}
	}
	close(quit)
	return nil
}

func submitTransaction(n *Node, tx []byte) error {
	n.proxy.SubmitCh() <- []byte(tx)
	return nil
}

func checkGossip(nodes []*Node, fromBlock int64, t *testing.T) {

	nodeBlocks := map[uint64][]poset.Block{}
	for _, n := range nodes {
		var blocks []poset.Block
		lastIndex := n.core.poset.GetLastBlockIndex()
		for i := fromBlock; i < lastIndex; i++ {
			block, err := n.core.poset.GetBlock(i)
			if err != nil {
				t.Fatalf("checkGossip: %v ", err)
			}
			blocks = append(blocks, block)
		}
		nodeBlocks[n.id] = blocks
	}

	minB := len(nodeBlocks[0])
	for k := uint64(1); k < uint64(len(nodes)); k++ {
		if len(nodeBlocks[k]) < minB {
			minB = len(nodeBlocks[k])
		}
	}

	for i, block := range nodeBlocks[0][:minB] {
		for k := uint64(1); k < uint64(len(nodes)); k++ {
			oBlock := nodeBlocks[k][i]
			if !reflect.DeepEqual(block.Body, oBlock.Body) {
				t.Fatalf("check gossip: difference in block %d."+
					" node 0: %v, node %d: %v",
					block.Index(), block.Body, k, oBlock.Body)
			}
		}
	}
}

func TestCreateAndInitNode(t *testing.T) {
	// Init data
	data := InitTestData(t, 1, 2)

	// Create transport
	trans := createTransport(t, data.Logger, data.BackConfig, data.Adds[0],
		data.PoolSize, data.CreateFu, data.Network.CreateListener)
	defer transportClose(t, trans)

	// Create & Init node
	node, posetWrapper := createNode(t, data.Logger, data.Config, data.PeersSlice[0].ID, data.Keys[0], data.Peers, trans, data.Adds[0], true)
	// Mock
	posetWrapper.EXPECT().Close().Return(nil).Times(1)

	// Check status
	nodeState := node.getState()
	if nodeState != Gossiping {
		t.Fatal(nodeState)
	}

	// Check ID
	if node.ID() != data.PeersSlice[0].ID {
		t.Fatal(node.id)
	}

	// Stop node & check status
	node.Stop()
	nodeState = node.getState()
	if nodeState != Stop {
		t.Fatal(nodeState)
	}

	// Shutdown node & check status
	node.Shutdown()
	nodeState = node.getState()
	if nodeState != Shutdown {
		t.Fatal(nodeState)
	}
}

func TestAddTransaction(t *testing.T) {
	// Init data
	data := InitTestData(t, 1, 2)

	// Create transport
	trans := createTransport(t, data.Logger, data.BackConfig, data.Adds[0],
		data.PoolSize, data.CreateFu, data.Network.CreateListener)
	defer transportClose(t, trans)

	// Create & Init node
	node, posetWrapper := createNode(t, data.Logger, data.Config, data.PeersSlice[0].ID, data.Keys[0], data.Peers, trans, data.Adds[0], true)
	// Mock
	posetWrapper.EXPECT().Close().Return(nil).Times(1)
	defer node.Shutdown()

	// Add new Tx
	message := "Test"
	err := node.core.AddTransactions([][]byte{[]byte(message)})
	if err != nil {
		t.Fatal(err.Error())
	}

	// Check tx pool
	txPoolCount := node.core.GetTransactionPoolCount()
	if txPoolCount != 1 {
		t.Fatal("Transaction pool count wrong")
	}

	// Add new Internal Tx
	internalTx := poset.InternalTransaction{}
	node.core.AddInternalTransactions([]poset.InternalTransaction{internalTx})

	// Check internal tx pool
	txPoolCount = node.core.GetInternalTransactionPoolCount()
	if txPoolCount != 1 {
		t.Fatal("Transaction pool count wrong")
	}
}

func TestTxHandler(t *testing.T) {
	// Init data
	data := InitTestData(t, 1, 2)

	// Create transport
	trans := createTransport(t, data.Logger, data.BackConfig, data.Adds[0],
		data.PoolSize, data.CreateFu, data.Network.CreateListener)
	defer transportClose(t, trans)

	// Create & Init node
	node, posetWrapper := createNode(t, data.Logger, data.Config, data.PeersSlice[0].ID, data.Keys[0], data.Peers, trans, data.Adds[0], true)
	defer node.Shutdown()

	// Mock for submitCh
	posetWrapper.EXPECT().InsertEvent(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	posetWrapper.EXPECT().SetWireInfoAndSign(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	posetWrapper.EXPECT().GetPendingLoadedEvents().Return(int64(1)).AnyTimes()

	posetWrapper.EXPECT().Close().Return(nil).Times(1)

	// Check submitCh case
	message := "Test"
	node.submitCh <- []byte(message)

	// Because of we need to wait to complete submitCh.
	time.Sleep(3 * time.Second)

	txPoolCount := node.core.GetTransactionPoolCount()
	if txPoolCount != 1 {
		t.Fatal("Transaction pool count wrong")
	}

	// Check submitInternalCh case
	internalTx := poset.InternalTransaction{}
	node.submitInternalCh <- internalTx

	// Because of we need to wait to complete submitInternalCh.
	time.Sleep(3 * time.Second)

	txPoolCount = node.core.GetInternalTransactionPoolCount()
	if txPoolCount != 1 {
		t.Fatal("Transaction pool count wrong")
	}
}


// TODO: Currently we get stuck on node2.addIntoPoset(data.PeersSlice[0], events) process for node1, probably we have issue with wrong mock.
func TestSyncProcess(t *testing.T) {
	// Init data
	data := InitTestData(t, 2, 2)

	// Create transport
	trans1 := createTransport(t, data.Logger, data.BackConfig, data.Adds[0],
		data.PoolSize, data.CreateFu, data.Network.CreateListener)
	defer transportClose(t, trans1)

	trans2 := createTransport(t, data.Logger, data.BackConfig, data.Adds[1],
		data.PoolSize, data.CreateFu, data.Network.CreateListener)
	defer transportClose(t, trans2)

	// Create & Init node
	node1, posetWrapper1 := createNode(t, data.Logger, data.Config, data.PeersSlice[0].ID, data.Keys[0], data.Peers, trans1, data.Adds[0], true)
	posetWrapper1.EXPECT().Close().Return(nil).Times(1)
	defer node1.Shutdown()

	node2, posetWrapper2 := createNode(t, data.Logger, data.Config, data.PeersSlice[1].ID, data.Keys[1], data.Peers, trans2, data.Adds[1], true)
	posetWrapper2.EXPECT().Close().Return(nil).Times(1)
	defer node2.Shutdown()

	// Mock for node2

	// Submit event process.
	posetWrapper2.EXPECT().InsertEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	posetWrapper2.EXPECT().SetWireInfoAndSign(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	posetWrapper2.EXPECT().GetPendingLoadedEvents().Return(int64(1)).AnyTimes()

	// TODO: Probably don't execute currently or we need to change return value.
	posetWrapper2.EXPECT().GetLastEvent(gomock.Any()).Return(&poset.EventHash{}, false, nil).AnyTimes()
	
	var index map[string]poset.EventHash
	posetWrapper2.EXPECT().GetParticipantEvents(gomock.Any(), gomock.Any()).Return(&poset.EventHashes{}, nil).AnyTimes()

	event := &poset.Event{
		Message: &poset.EventMessage{
			Body: &poset.EventBody{
				Index: 0,
			},
			TopologicalIndex: 0,
		},
	}
	posetWrapper2.EXPECT().GetEventBlock(gomock.Any()).Return(event, nil).AnyTimes()
	
	posetWrapper2.EXPECT().GetLastBlockIndex().Return(int64(0)).AnyTimes()
	posetWrapper2.EXPECT().GetBlock(gomock.Any()).Return(poset.Block{}, nil).AnyTimes()
	posetWrapper2.EXPECT().NewEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(*event).AnyTimes() // TODO: Return event with known creator.

	// Mock for node1

	// Submit event process.
	posetWrapper1.EXPECT().InsertEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	posetWrapper1.EXPECT().SetWireInfoAndSign(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	posetWrapper1.EXPECT().GetPendingLoadedEvents().Return(int64(1)).AnyTimes()

	// TODO: Probably don't execute currently or we need to change return value.
	posetWrapper1.EXPECT().GetLastEvent(gomock.Any()).Return(&poset.EventHash{}, false, nil).AnyTimes()
	
	posetWrapper1.EXPECT().GetParticipantEvents(gomock.Any(), gomock.Any()).Return(&poset.EventHashes{index["e4"], index["e03"]}, nil).AnyTimes()

	posetWrapper1.EXPECT().GetEventBlock(gomock.Any()).Return(event, nil).AnyTimes()
	
	posetWrapper1.EXPECT().GetLastBlockIndex().Return(int64(0)).AnyTimes()
	posetWrapper1.EXPECT().GetBlock(gomock.Any()).Return(poset.Block{}, nil).AnyTimes()

	// Submit transaction for node2
	message := "Test"
	node2.submitCh <- []byte(message)

	// Get data from node1
	events, heights, err := node2.getUnknownEventsFromPeer(data.PeersSlice[0])
	if err != nil {
		t.Fatalf(err.Error())
	}

	if l := len(*events); l != 0 {
		t.Fatalf("expected %d, got %d", 0, l)
	}

	// Add data into poset (Create new event with tx)
	err = node2.addIntoPoset(data.PeersSlice[0], events)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// Collect data for node1 from node2
	events, err = node2.collectUnknownEventsForPeer(data.PeersSlice[0], heights)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if l := len(*events); l != 1 {
		t.Fatalf("expected %d, got %d", 1, l)
	}

	// Send data from node2 to node1
	_, err = node2.requestWithEvents(data.PeersSlice[0].NetAddr, *events)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// We need to wait while node1 accept & process new request.
	time.Sleep(3 * time.Second)

	// Check tx pool for node1 & node2
	if l := len(node1.core.transactionPool); l > 0 {
		t.Fatalf("expected %d, got %d", 0, l)
	}

	if l := len(node2.core.transactionPool); l > 0 {
		t.Fatalf("expected %d, got %d", 0, l)
	}

	// Get head & check tx count
	node1Head, err := node1.core.GetHead()
	if err != nil {
		t.Fatal(err)
	}

	if l := len(node1Head.Transactions()); l == 1 {
		t.Fatalf("expected %d, got %d", 0, l)
	}

	// Get head & check tx count
	node2Head, err := node2.core.GetHead()
	if err != nil {
		t.Fatal(err)
	}

	if l := len(node2Head.Transactions()); l != 1 {
		t.Fatalf("expected %d, got %d", 1, l)
	}

	// Check message
	if m := string(node2Head.Transactions()[0]); m != message {
		t.Fatalf("expected message %s, got %s", message, m)
	}
}
