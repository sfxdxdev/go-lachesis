package net

import (
	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

// SyncRequest initiates a synchronization request
type SyncRequest struct {
	FromID common.Address
	Known  map[common.Address]int64
}

// SyncResponse is a response to a SyncRequest request
type SyncResponse struct {
	FromID    common.Address
	SyncLimit bool
	Events    []poset.WireEvent
	Known     map[common.Address]int64
}

//++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

// EagerSyncRequest after an initial sync to quickly catch up
type EagerSyncRequest struct {
	FromID common.Address
	Events []poset.WireEvent
}

// EagerSyncResponse response to an EagerSyncRequest
type EagerSyncResponse struct {
	FromID  common.Address
	Success bool
}

//++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

// FastForwardRequest request to start a fast forward catch up
type FastForwardRequest struct {
	FromID common.Address
}

// FastForwardResponse response with the snapshot data for fast forward request
type FastForwardResponse struct {
	FromID   common.Address
	Block    poset.Block
	Frame    poset.Frame
	Snapshot []byte
}
