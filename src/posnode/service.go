package posnode

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/posnode/network"
	"github.com/Fantom-foundation/go-lachesis/src/posnode/wire"
)

type service struct {
	server *grpc.Server
}

// StartService starts node service.
// It should be called once.
func (n *Node) StartService() {
	bind := net.JoinHostPort(n.host, strconv.Itoa(n.conf.Port))
	listener := network.TcpListener(bind)
	n.startService(listener)
}

func (n *Node) startService(listener net.Listener) {
	n.server = grpc.NewServer(
		grpc.MaxRecvMsgSize(math.MaxInt32),
		grpc.MaxSendMsgSize(math.MaxInt32))
	wire.RegisterNodeServer(n.server, n)

	n.log.Infof("service start at %v", listener.Addr())
	go func() {
		if err := n.server.Serve(listener); err != nil {
			n.log.Infof("service stop (%v)", err)
		}
	}()
}

// StopService stops node service.
// It should be called once.
func (n *Node) StopService() {
	n.server.GracefulStop()
}

/*
 * wire.NodeServer implementation:
 */

// SyncEvents it remember their known events for future request
// and returns unknown for they events.
func (n *Node) SyncEvents(ctx context.Context, req *wire.KnownEvents) (*wire.KnownEvents, error) {
	knownHeights := n.store_GetHeights()

	result := map[string]uint64{}

	// Collect data about known peer from another node & add unknown peer to store
	for pID, height := range req.Lasts {
		// Check data about known peer
		if knownValue, ok := (*knownHeights).Lasts[pID]; ok {
			if knownValue > height {
				result[pID] = knownValue
			} else if knownValue < height { // if equal -> do nothing
				(*knownHeights).Lasts[pID] = height
			}
		} else {
			// if unknown peer -> add to store
			(*knownHeights).Lasts[pID] = height
		}
	}

	// Collect unknown peers for another node
	for pID, height := range (*knownHeights).Lasts {
		if _, ok := req.Lasts[pID]; !ok {
			result[pID] = height
		}
	}

	n.store_SetHeights(knownHeights)

	return &wire.KnownEvents{Lasts: result}, nil
}

// GetEvent returns requested event.
func (n *Node) GetEvent(ctx context.Context, req *wire.EventRequest) (*wire.Event, error) {
	// TODO: implement it
	return nil, nil
}

// GetPeerInfo returns requested peer info.
func (n *Node) GetPeerInfo(ctx context.Context, req *wire.PeerRequest) (*wire.PeerInfo, error) {
	id := common.HexToAddress(req.PeerID)
	peerInfo := n.store.GetPeerInfo(id)
	// peerInfo == nil means that peer not found
	// by given id
	if peerInfo == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("peer not found: %s", req.PeerID))
	}

	return peerInfo, nil
}