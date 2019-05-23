package validators

import (
	"github.com/sirupsen/logrus"

	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
)

/*
 * Event's parents validator:
 */

// EventsOrderingValidator checks parent nodes rule.
type EventsOrderingValidator struct {
	event   *inter.Event
	nodes   map[hash.Peer]struct{}
	maxTime inter.Timestamp

	log *logrus.Logger
}

func NewEventsOrderingValidator(e *inter.Event, log *logrus.Logger) *EventsOrderingValidator {
	n := make(map[hash.Peer]struct{}, len(e.Parents))

	return &EventsOrderingValidator{
		event:   e,
		nodes:   n,
		maxTime: 0,

		log: log,
	}
}

func (v *EventsOrderingValidator) IsValidTime(node hash.Peer, t inter.Timestamp) bool {
	return v.IsParentUnique(node) && v.IsGreaterThan(t)
}

func (v *EventsOrderingValidator) IsValidCreator() bool {
	return v.HasSelfParent() && v.IsSequential()
}

func (v *EventsOrderingValidator) IsGreaterThan(time inter.Timestamp) bool {
	if v.event.LamportTime <= time {
		v.log.Warnf("Event %s has lamport time %d. It isn't next of parents, so rejected",
			v.event.Hash().String(),
			v.event.LamportTime)
		return false
	}
	if v.maxTime < time {
		v.maxTime = time
	}
	return true
}

func (v *EventsOrderingValidator) IsSequential() bool {
	if v.event.LamportTime != v.maxTime+1 {
		v.log.Warnf("Event %s has lamport time %d. It is too far from parents, so rejected",
			v.event.Hash().String(),
			v.event.LamportTime)
		return false
	}
	return true
}

func (v *EventsOrderingValidator) IsParentUnique(node hash.Peer) bool {
	if _, ok := v.nodes[node]; ok {
		eventName, peerName := v.event.Hash().String(), node.String()
		v.log.Warnf("Event %s has double refer to node %s, so rejected", eventName, peerName)
		return false
	}
	v.nodes[node] = struct{}{}
	return true

}

func (v *EventsOrderingValidator) HasSelfParent() bool {
	if _, ok := v.nodes[v.event.Creator]; !ok {
		eventName, peerName := v.event.Hash().String(), v.event.Creator.String()
		v.log.Warnf("Event %s has no refer to self-node %s, so rejected", eventName, peerName)
		return false
	}
	return true
}
