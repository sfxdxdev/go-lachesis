package node2

import (
	"context"
	"fmt"

	"github.com/Fantom-foundation/go-lachesis/src/peer"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

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
