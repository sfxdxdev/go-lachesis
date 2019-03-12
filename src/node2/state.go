package node2

import (
	"sync"
)

const (
	// Gossiping is the initial state of a Lachesis node.
	Gossiping state = iota
	// Shutdown is the shut down state
	Shutdown
	// Stop is the stop communicating state
	Stop
)

type state int

type nodeState struct {
	cond *sync.Cond
	lock sync.RWMutex
	wip  int

	state        state
	getStateChan chan state
	setStateChan chan state
}

func newNodeState() *nodeState {
	ns := &nodeState{
		cond:         sync.NewCond(&sync.Mutex{}),
		getStateChan: make(chan state),
		setStateChan: make(chan state),
	}
	go ns.mtx()
	return ns
}

func (s state) String() string {
	switch s {
	case Gossiping:
		return "Gossiping"
	case Shutdown:
		return "Shutdown"
	case Stop:
		return "Stop"
	default:
		return "Unknown"
	}
}

func (s *nodeState) mtx() {
	for {
		select {
		case s.state = <-s.setStateChan:
		case s.getStateChan <- s.state:
		}
	}
}

func (s *nodeState) goFunc(fu func()) {
	go func() {
		s.lock.Lock()
		s.wip++
		s.lock.Unlock()

		fu()

		s.lock.Lock()
		s.wip--
		s.lock.Unlock()

		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		s.cond.Broadcast()
	}()
}

func (s *nodeState) waitRoutines() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	for {
		s.lock.RLock()
		wip := s.wip
		s.lock.RUnlock()

		if wip != 0 {
			s.cond.Wait()
			continue
		}
		break
	}
}

func (s *nodeState) getState() state {
	return <-s.getStateChan
}

func (s *nodeState) setState(state state) {
	s.setStateChan <- state
}
