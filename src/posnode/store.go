package posnode

import (
	"github.com/dgraph-io/badger"
	"github.com/golang/protobuf/proto"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/kvdb"
	"github.com/Fantom-foundation/go-lachesis/src/posnode/wire"
)

// Store is a node persistent storage working over physical key-value database.
type Store struct {
	physicalDB kvdb.Database

	peers       kvdb.Database
	topPeers    kvdb.Database
	knownPeers  kvdb.Database
	peerHeights kvdb.Database
}

// NewMemStore creates store over memory map.
func NewMemStore() *Store {
	s := &Store{
		physicalDB: kvdb.NewMemDatabase(),
	}
	s.init()
	return s
}

// NewBadgerStore creates store over badger database.
func NewBadgerStore(db *badger.DB) *Store {
	s := &Store{
		physicalDB: kvdb.NewBadgerDatabase(db),
	}
	s.init()
	return s
}

func (s *Store) init() {
	s.peers = kvdb.NewTable(s.physicalDB, "peer_")
	s.topPeers = kvdb.NewTable(s.physicalDB, "top_peers_")
	s.knownPeers = kvdb.NewTable(s.physicalDB, "known_peers_")
	s.peerHeights = kvdb.NewTable(s.physicalDB, "peer_height_")
}

// Close leaves underlying database.
func (s *Store) Close() {
	s.peerHeights = nil
	s.knownPeers = nil
	s.topPeers = nil
	s.peers = nil
	s.physicalDB.Close()
}

// SetPeer stores peer.
func (s *Store) SetPeer(peer *Peer) {
	w := peer.ToWire()
	s.set(s.peers, peer.ID.Bytes(), w)
}

// GetPeerInfo returns stored peer info.
// Result is a ready gRPC message.
func (s *Store) GetPeerInfo(id common.Address) *wire.PeerInfo {
	w, _ := s.get(s.peers, id.Bytes(), &wire.PeerInfo{}).(*wire.PeerInfo)
	return w
}

// GetPeer returns stored peer.
func (s *Store) GetPeer(id common.Address) *Peer {
	w := s.GetPeerInfo(id)
	if w == nil {
		return nil
	}

	return WireToPeer(w)
}

// SetTopPeers stores peers.top.
func (s *Store) SetTopPeers(ids []common.Address) {
	var key = []byte("current")
	w := IDsToWire(ids)
	s.set(s.topPeers, key, w)
}

// GetTopPeers returns peers.top.
func (s *Store) GetTopPeers() []common.Address {
	var key = []byte("current")
	w, _ := s.get(s.topPeers, key, &wire.PeersID{}).(*wire.PeersID)
	return WireToIDs(w)
}

// SetKnownPeers stores all peers ID.
func (s *Store) SetKnownPeers(ids []common.Address) {
	var key = []byte("current")
	w := IDsToWire(ids)
	s.set(s.knownPeers, key, w)
}

// GetKnownPeers returns all peers ID.
func (s *Store) GetKnownPeers() []common.Address {
	var key = []byte("current")
	w, _ := s.get(s.knownPeers, key, &wire.PeersID{}).(*wire.PeersID)
	return WireToIDs(w)
}

// SetPeerHeight stores last event index of peer.
func (s *Store) SetPeerHeight(id common.Address, height uint64) {
	if err := s.peerHeights.Put(id.Bytes(), intToBytes(height)); err != nil {
		panic(err)
	}
}

// GetPeerHeight returns last event index of peer.
func (s *Store) GetPeerHeight(id common.Address) uint64 {
	buf, err := s.peerHeights.Get(id.Bytes())
	if err != nil {
		panic(err)
	}
	if buf == nil {
		return 0
	}

	return bytesToInt(buf)
}

/*
 * Utils:
 */

func (s *Store) set(table kvdb.Database, key []byte, val proto.Message) {
	var pbf proto.Buffer

	if err := pbf.Marshal(val); err != nil {
		panic(err)
	}

	if err := table.Put(key, pbf.Bytes()); err != nil {
		panic(err)
	}
}

func (s *Store) get(table kvdb.Database, key []byte, to proto.Message) proto.Message {
	buf, err := table.Get(key)
	if err != nil {
		panic(err)
	}
	if buf == nil {
		return nil
	}

	err = proto.Unmarshal(buf, to)
	if err != nil {
		panic(err)
	}
	return to
}

func (s *Store) has(table kvdb.Database, key []byte) bool {
	res, err := table.Has(key)
	if err != nil {
		panic(err)
	}
	return res
}

func intToBytes(n uint64) []byte {
	var res [8]byte
	for i := 0; i < len(res); i++ {
		res[i] = byte(n)
		n = n >> 8
	}
	return res[:]
}

func bytesToInt(b []byte) uint64 {
	var res uint64
	for i := 0; i < len(b); i++ {
		res += uint64(b[i]) << uint(i*8)
	}
	return res
}