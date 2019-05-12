package posnode

import (
	"sync"

	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
	"github.com/Fantom-foundation/go-lachesis/src/posnode/validators"
)

type incompleteEvents map[hash.Event]*ordering

type events struct {
	wg   sync.WaitGroup
	exit chan struct{}

	unsortedEventCh chan *inter.Event
	incomplete      incompleteEvents
}

type order map[hash.Event]*ordering

type ordering struct {
	*inter.Event

	time    inter.Timestamp
	parents map[hash.Event]*ordering
}

func NewOrderingEvents() *events {
	const buffSize = 10

	return &events{
		exit:            make(chan struct{}, 1),
		unsortedEventCh: make(chan *inter.Event, buffSize),
		incomplete:      make(incompleteEvents),
	}
}

func (n *Node) ToOrdering(event *inter.Event) {
	e := n.events
	e.unsortedEventCh <- event
}

func (n *Node) OrderingEventsStart() {
	e := n.events
	if e.exit != nil {
		return
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			select {
			case <-e.exit:
				return
			case unsortedEvent := <-e.unsortedEventCh:
				n.orderingEvents(unsortedEvent)
			}
		}
	}()
}

func (n *Node) OrderingEventsStop() {
	e := n.events
	if e.exit == nil {
		return
	}

	close(e.exit)
	e.wg.Wait()
	e.exit = nil
}

// orderingEvents is a wrapper for sorting inside the posnode package
func (n *Node) orderingEvents(event *inter.Event) {
	if n.consensus != nil && event != nil {
		n.ordering(event)
	}
}

// ordering is a wrapper for initializing the sorting of a new event
func (n *Node) ordering(event *inter.Event) {
	if n.consensus.GetEventFrame(event.Hash()) != nil {
		log.WithField("event", event).Warnf("Event had received already")
		return
	}

	orderingEvents := &ordering{
		Event:   event,
		parents: make(order, len(event.Parents)),
		time:    0,
	}

	for eventHash := range event.Parents {
		orderingEvents.parents[eventHash] = nil
	}

	n.sortEvents(orderingEvents)
}

func (n *Node) sortEvents(o *ordering) {
	event := o.Event
	validate := validators.NewEventsOrderingValidator(event)

	// fill in the parent's index of the event or consider it incomplete
	for pHash := range event.Parents {
		if pHash.IsZero() {
			// first node event
			if !validate.IsValidTime(event.Creator, 0) {
				return
			}
			continue
		}

		parent := o.parents[pHash]
		if parent == nil {
			if n.consensus.GetEventFrame(pHash) == nil {
				n.events.incomplete[o.Hash()] = o
				return
			}

			parent = &ordering{
				Event: n.store.GetEvent(pHash),
			}

			o.parents[pHash] = parent
		}
		if !validate.IsValidTime(parent.Creator, parent.LamportTime) {
			return
		}
	}

	if !validate.IsValidCreator() {
		return
	}

	// parent's events indexes validated
	n.consensus.Consensus(event)

	// child events can now complete, check it again
	for pHash, child := range n.events.incomplete {
		if parent, ok := child.parents[event.Hash()]; ok && parent == nil {
			child.parents[event.Hash()] = o
			delete(n.events.incomplete, pHash)
			n.sortEvents(o)
		}
	}
}
