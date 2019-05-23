package sorting

import (
	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
	"github.com/Fantom-foundation/go-lachesis/src/inter/validators"
)

type Source interface {
	GetEvent(hash.Event) *inter.Event
	SetEvent(e *inter.Event)
}

type Ordering struct {
	source Source
	log    *logrus.Logger
	async  bool

	incompleteEvents map[hash.Event]*internalEvent
}

type internalEvent struct {
	inter.Event

	consensusTime inter.Timestamp
	parents       map[hash.Event]*internalEvent
}

type eventStream struct {
	event *internalEvent

	out   chan *inter.Event
	exit  chan struct{}
	ready chan struct{}

	async bool
}

type callbacks func(event *inter.Event)
type verifiers func(event hash.Event) *uint64

func NewOrder(source Source, log *logrus.Logger, async ...bool) *Ordering {
	order := &Ordering{
		source: source,
		log:    log,

		incompleteEvents: make(map[hash.Event]*internalEvent),
	}
	if len(async) > 0 {
		order.async = async[0]
	}
	return order
}

func (o *Ordering) NewEventConsumer(e *inter.Event, cb ...callbacks) *eventStream {
	event := &internalEvent{
		Event:         *e,
		consensusTime: 0,
		parents:       make(map[hash.Event]*internalEvent, len(e.Parents)),
	}
	for pHash := range e.Parents {
		event.parents[pHash] = nil
	}

	ch := &eventStream{
		event: event,

		out:   make(chan *inter.Event),
		exit:  make(chan struct{}, 1),
		ready: make(chan struct{}, 1),

		async: o.async,
	}
	consumer(ch, cb...)
	return ch
}

func consumer(ch *eventStream, cb ...callbacks) {
	if ch == nil {
		return
	}

	go worker(ch, cb...)
}

func worker(ch *eventStream, cb ...callbacks) {
	async := ch.async
	for {
		select {
		case event := <-ch.out:
			for i := range cb {
				cb[i](event)
			}
			if !async {
				ch.ready <- struct{}{}
			}
		case <-ch.exit:
			return
		}
	}
}

func (o *Ordering) NewEventOrder(ch *eventStream, vf ...verifiers) {
	e := ch.event
	for i := range vf {
		if vf[i](e.Hash()) != nil {
			o.log.WithField("event", e).Warnf("Event had received already")
			return
		}
	}

	o.newEvent(ch, vf...)
}

func (o *Ordering) newEvent(ch *eventStream, vf ...verifiers) {
	e := ch.event
	source := o.source
	validate := validators.NewEventsOrderingValidator(&e.Event, o.log)

	// fill event's parents index or hold it as incompleted
	for pHash := range e.Parents {
		if pHash.IsZero() {
			// first node event
			if !validate.IsValidTime(e.Creator, 0) {
				return
			}
			continue
		}
		parent := e.parents[pHash]
		if parent == nil {
			p := source.GetEvent(pHash)
			if p == nil {
				o.incompleteEvents[e.Hash()] = e
				return
			}
			for i := range vf {
				if vf[i](pHash) == nil {
					o.incompleteEvents[e.Hash()] = e
					return
				}
			}
			parent = &internalEvent{
				Event: *p,
			}
			e.parents[pHash] = parent
		}

		if !validate.IsValidTime(parent.Creator, parent.LamportTime) {
			return
		}
	}

	if !validate.IsValidCreator() {
		return
	}

	// parents OK
	ch.out <- &e.Event
	if !ch.async {
		<-ch.ready
	}

	incomplete := o.incompleteEvents
	if len(incomplete) == 0 {
		ch.exit <- struct{}{}
	}

	for pHash, child := range incomplete {
		if parent, ok := child.parents[e.Hash()]; ok && parent == nil {
			child.parents[e.Hash()] = e
			delete(incomplete, pHash)
			ch.event = child
			o.newEvent(ch, vf...)
		}
	}
}
