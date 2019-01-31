package net

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/poset"
)

func testTransportImplementation(t *testing.T, trans1, trans2 Transport) {
	var wg sync.WaitGroup
	timeout := 200 * time.Millisecond

	// Transport 1 is consumer
	rpcCh := trans1.Consumer()

	t.Run("Sync", func(t *testing.T) {
		assert := assert.New(t)

		expectedReq := &SyncRequest{
			FromID: fakeAddr(0),
			Known: map[common.Address]int64{
				fakeAddr(0): 1,
				fakeAddr(1): 2,
				fakeAddr(2): 3,
			},
		}

		expectedResp := &SyncResponse{
			FromID: fakeAddr(1),
			Events: []poset.WireEvent{
				poset.WireEvent{
					Body: poset.WireBody{
						Transactions:         [][]byte(nil),
						SelfParentIndex:      1,
						OtherParentCreatorID: []byte{10},
						OtherParentIndex:     0,
						CreatorID:            []byte{9},
					},
				},
			},
			Known: map[common.Address]int64{
				fakeAddr(0): 5,
				fakeAddr(1): 5,
				fakeAddr(2): 6,
			},
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case rpc := <-rpcCh:
				req := rpc.Command.(*SyncRequest)
				assert.EqualValues(expectedReq, req)
				rpc.Respond(expectedResp, nil)
			case <-time.After(timeout):
				assert.Fail("timeout")
			}
		}()

		var resp = new(SyncResponse)
		err := trans2.Sync(trans1.LocalAddr(), expectedReq, resp)
		if assert.NoError(err) {
			assert.EqualValues(expectedResp, resp)
		}
		wg.Wait()
	})

	t.Run("EagerSync", func(t *testing.T) {
		assert := assert.New(t)

		expectedReq := &EagerSyncRequest{
			FromID: fakeAddr(0),
			Events: []poset.WireEvent{
				poset.WireEvent{
					Body: poset.WireBody{
						Transactions:         [][]byte(nil),
						SelfParentIndex:      1,
						OtherParentCreatorID: []byte{10},
						OtherParentIndex:     0,
						CreatorID:            []byte{9},
					},
				},
			},
		}

		expectedResp := &EagerSyncResponse{
			FromID:  fakeAddr(1),
			Success: true,
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case rpc := <-rpcCh:
				req := rpc.Command.(*EagerSyncRequest)
				assert.EqualValues(expectedReq, req)
				rpc.Respond(expectedResp, nil)
			case <-time.After(timeout):
				assert.Fail("timeout")
			}
		}()

		var resp = new(EagerSyncResponse)
		err := trans2.EagerSync(trans1.LocalAddr(), expectedReq, resp)
		if assert.NoError(err) {
			assert.EqualValues(expectedResp, resp)
		}
		wg.Wait()
	})

	t.Run("FastForward", func(t *testing.T) {
		assert := assert.New(t)

		expectedReq := &FastForwardRequest{
			FromID: fakeAddr(0),
		}

		frame := poset.Frame{}
		block, err := poset.NewBlockFromFrame(1, frame)
		assert.NoError(err)
		expectedResp := &FastForwardResponse{
			FromID:   fakeAddr(1),
			Block:    block,
			Frame:    frame,
			Snapshot: []byte("snapshot"),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case rpc := <-rpcCh:
				req := rpc.Command.(*FastForwardRequest)
				assert.EqualValues(expectedReq, req)
				rpc.Respond(expectedResp, nil)
			case <-time.After(timeout):
				assert.Fail("timeout")
			}
		}()

		var resp = new(FastForwardResponse)
		err = trans2.FastForward(trans1.LocalAddr(), expectedReq, resp)
		if resp.Block.Signatures == nil {
			resp.Block.Signatures = make(map[string]string)
		}
		if assert.NoError(err) {
			assert.EqualValues(expectedResp, resp)
		}
		wg.Wait()
	})
}
