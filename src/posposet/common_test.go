package posposet

import (
	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
	"github.com/Fantom-foundation/go-lachesis/src/inter/sorting"
)

// FakePoset creates empty poset with mem store and equal stakes of nodes in genesis.
func FakePoset(nodes []hash.Peer) (*Poset, *Store, *EventStore) {
	balances := make(map[hash.Peer]uint64, len(nodes))
	for _, addr := range nodes {
		balances[addr] = uint64(1)
	}

	store := NewMemStore()
	err := store.ApplyGenesis(balances)
	if err != nil {
		panic(err)
	}

	input := NewEventStore(nil, false)

	poset := New(store, input)
	poset.Bootstrap()

	return poset, store, input
}

// FakePosetWithOrdering creates empty poset with mem store, ordering struct and equal stakes of nodes in genesis.
func FakePosetWithOrdering(nodes []hash.Peer) (*Poset, *Store, *EventStore, *sorting.Ordering) {
	balances := make(map[hash.Peer]uint64, len(nodes))
	for _, addr := range nodes {
		balances[addr] = uint64(1)
	}

	store := NewMemStore()
	err := store.ApplyGenesis(balances)
	if err != nil {
		panic(err)
	}

	input := NewEventStore(nil, false)

	poset := New(store, input)
	poset.Bootstrap()
	order := sorting.NewOrder(input, log)

	return poset, store, input, order
}

// GenEventsByNode generates random events for test purpose.
// Result:
//   - nodes  is an array of node addresses;
//   - events maps node address to array of its events;
// It wraps inter.GenEventsByNode()
func GenEventsByNode(nodeCount, eventCount, parentCount int) (
	nodes []hash.Peer, events map[hash.Peer][]*Event) {

	var inters map[hash.Peer][]*inter.Event

	nodes, inters = inter.GenEventsByNode(nodeCount, eventCount, parentCount)
	// wrap inter.Event into Event
	events = make(map[hash.Peer][]*Event, len(inters))
	for h, from := range inters {
		to := make([]*Event, len(from))
		for i, e := range from {
			to[i] = &Event{Event: *e}
		}
		events[h] = to
	}

	return
}
