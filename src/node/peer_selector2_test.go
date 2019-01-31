package node

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
)

func TestSmartSelectorEmpty(t *testing.T) {
	assert := assert.New(t)

	fp := fakePeers(0)

	ss := NewSmartPeerSelector(
		fp,
		"",
		func() (map[common.Address]int64, error) {
			return nil, nil
		},
	)

	assert.Nil(ss.Next())
}

func TestSmartSelectorLocalAddrOnly(t *testing.T) {
	assert := assert.New(t)

	fp := fakePeers(1)
	fps := fp.ToPeerSlice()

	ss := NewSmartPeerSelector(
		fp,
		fps[0].NetAddr,
		func() (map[common.Address]int64, error) {
			return nil, nil
		},
	)

	assert.Nil(ss.Next())
}

func TestSmartSelectorUsed(t *testing.T) {
	assert := assert.New(t)

	fp := fakePeers(3)
	fps := fp.ToPeerSlice()

	ss := NewSmartPeerSelector(
		fp,
		fps[0].NetAddr,
		func() (map[common.Address]int64, error) {
			return nil, nil
		},
	)

	choose1 := ss.Next().NetAddr
	assert.NotEqual(fps[0].NetAddr, choose1)

	choose2 := ss.Next().NetAddr
	assert.NotEqual(fps[0].NetAddr, choose2)
	assert.NotEqual(choose1, choose2)

	choose3 := ss.Next().NetAddr
	assert.NotEqual(fps[0].NetAddr, choose3)
}

func TestSmartSelectorFlagged(t *testing.T) {
	assert := assert.New(t)

	fp := fakePeers(3)
	fps := fp.ToPeerSlice()

	ss := NewSmartPeerSelector(
		fp,
		fps[0].NetAddr,
		func() (map[common.Address]int64, error) {
			return map[common.Address]int64{
				fps[2].ID: 1,
			}, nil
		},
	)

	assert.Equal(fps[1].NetAddr, ss.Next().NetAddr)
	assert.Equal(fps[1].NetAddr, ss.Next().NetAddr)
	assert.Equal(fps[1].NetAddr, ss.Next().NetAddr)
}

func TestSmartSelectorGeneral(t *testing.T) {
	assert := assert.New(t)

	fp := fakePeers(4)
	fps := fp.ToPeerSlice()

	ss := NewSmartPeerSelector(
		fp,
		fps[3].NetAddr,
		func() (map[common.Address]int64, error) {
			return map[common.Address]int64{
				fps[0].ID: 0,
				fps[1].ID: 0,
				fps[2].ID: 1,
				fps[3].ID: 0,
			}, nil
		},
	)

	addresses := []string{fps[0].NetAddr, fps[1].NetAddr}
	assert.Contains(addresses, ss.Next().NetAddr)
	assert.Contains(addresses, ss.Next().NetAddr)
	assert.Contains(addresses, ss.Next().NetAddr)
	assert.Contains(addresses, ss.Next().NetAddr)
}

/*
 * go test -bench "BenchmarkSmartSelectorNext" -benchmem -run "^$" ./src/node
 */

func BenchmarkSmartSelectorNext(b *testing.B) {
	const fakePeersCount = 50

	participants1 := fakePeers(fakePeersCount)
	participants2 := clonePeers(participants1)

	flagTable1 := fakeFlagTable(participants1)

	ss1 := NewSmartPeerSelector(
		participants1,
		fakeNetAddr(0),
		func() (map[common.Address]int64, error) {
			return flagTable1, nil
		},
	)
	rnd := NewRandomPeerSelector(
		participants2,
		fakeNetAddr(0),
	)

	b.ResetTimer()

	b.Run("smart Next()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := ss1.Next()
			if p == nil {
				b.Fatal("No next peer")
				break
			}
			ss1.UpdateLast(p)
		}
	})

	b.Run("simple Next()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := rnd.Next()
			if p == nil {
				b.Fatal("No next peer")
				break
			}
			rnd.UpdateLast(p)
		}
	})

}

/*
 * stuff
 */

func fakeFlagTable(participants *peers.Peers) map[common.Address]int64 {
	res := make(map[common.Address]int64, participants.Len())
	for id := range participants.ByID {
		res[id] = rand.Int63n(2)
	}
	return res
}
