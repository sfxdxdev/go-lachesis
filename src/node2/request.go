package node2

import (
	"context"
	"fmt"
	"time"

	"github.com/Fantom-foundation/go-lachesis/src/peer"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

func (n *Node) txHandler() {
	for {
		select {
		case tx := <-n.submitCh:
			err := n.core.AddTransactions([][]byte{tx})
			if err != nil {
				n.logger.Errorf("Adding Transactions to Transaction Pool: %s", err)
			}

			n.resetTimer()
		case tx := <-n.submitInternalCh:
			n.core.AddInternalTransactions([]poset.InternalTransaction{tx})
			n.resetTimer()
		}
	}
}

func (n *Node) requestHandler() {
	for {
		select {
		case rpc, ok := <-n.trans.ReceiverChannel():
			if !ok {
				return
			}

			n.goFunc(func() {
				n.rpcJobs.increment()
				n.processRequest(rpc)
				n.resetTimer()
				n.rpcJobs.decrement()
			})
		case <-n.controlTimer.tickCh:
			if n.gossipJobs.get() < 1 {
				n.goFunc(func() {
					n.gossipJobs.increment()
					
					participants := n.core.participants.ToPeerSlice()

					for i := 0; i < len(participants); i++ {
						go n.gossip(participants[i])
					}
					
					n.gossipJobs.decrement()
				})
			}
			
			n.resetTimer()
		case <-n.shutdownCh:
			return
		}
	}
}

func (n *Node) resetTimer() {
	if !n.controlTimer.GetSet() {
		ts := n.conf.HeartbeatTimeout

		// Slow gossip if nothing interesting to say
		if n.core.poset.GetPendingLoadedEvents() == 0 &&
			n.core.GetTransactionPoolCount() == 0 &&
			n.core.GetBlockSignaturePoolCount() == 0 {
			ts = time.Duration(time.Second)
		}

		n.controlTimer.resetCh <- ts
	}
}

func (n *Node) processRequest(rpc *peer.RPC) {
	switch cmd := rpc.Command.(type) {
	case *peer.SyncRequest:
		n.processRequestWithHeights(rpc, cmd)
	case *peer.ForceSyncRequest:
		n.processRequestWithEvents(rpc, cmd)
	default:
		// TODO: context.Background
		rpc.SendResult(context.Background(), n.logger,
			nil, fmt.Errorf("unexpected command"))
	}
}

func (n *Node) requestWithEvents(target string, events []poset.WireEvent) (*peer.ForceSyncResponse, error) {
	args := &peer.ForceSyncRequest{FromID: n.id, Events: events}
	out := &peer.ForceSyncResponse{}
	err := n.trans.ForceSync(context.Background(), target, args, out)

	return out, err
}

func (n *Node) requestWithHeights(target string, heights map[uint64]int64) (*peer.SyncResponse, error) {
	args := &peer.SyncRequest{FromID: n.id, Known: heights}
	out := &peer.SyncResponse{}
	err := n.trans.Sync(context.Background(), target, args, out)

	return out, err
}

func (n *Node) processRequestWithEvents(rpc *peer.RPC, cmd *peer.ForceSyncRequest) {
	success := true
	participants, err := n.core.poset.Store.Participants()
	if err != nil {
		success = false
	}
	p, ok := participants.ReadByID(cmd.FromID)
	if !ok {
		success = false
	}

	n.coreLock.Lock()
	err = n.addIntoPoset(&p, &cmd.Events)
	if err != nil {
		success = false
	}
	n.coreLock.Unlock()

	resp := &peer.ForceSyncResponse{
		FromID:  n.id,
		Success: success,
	}

	// TODO: context.Background
	rpc.SendResult(context.Background(), n.logger, resp, nil)
}

func (n *Node) processRequestWithHeights(rpc *peer.RPC, cmd *peer.SyncRequest) {
	var respErr error

	resp := &peer.SyncResponse{
		FromID: n.id,
	}

	// Get unknown events for source node
	n.coreLock.Lock()
	eventDiff, err := n.core.EventDiff(cmd.Known)
	if err != nil {
		respErr = err
	}
	n.coreLock.Unlock()

	// Convert to WireEvents
	if len(eventDiff) > 0 {
		wireEvents := n.core.ToWire(eventDiff)
		resp.Events = wireEvents
	} else {
		resp.Events = []poset.WireEvent{}
	}

	// Get Self Known
	n.coreLock.Lock()
	knownHeights := n.core.GetKnownHeights()
	n.coreLock.Unlock()

	resp.Known = knownHeights

	// TODO: context.Background
	rpc.SendResult(context.Background(), n.logger, resp, respErr)
}
