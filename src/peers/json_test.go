package peers

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Fantom-foundation/go-lachesis/src/crypto"
)

func TestJSONPeers(t *testing.T) {
	const N = 3
	assert := assert.New(t)

	// Create a test dir
	dir, err := ioutil.TempDir("", "lachesis_")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	// Create the store
	var store PeerStore

	store = NewJSONPeers(dir)

	// Try a read, should get nothing
	peers, err := store.Read()
	assert.Error(err)
	assert.Nil(peers)

	keys := map[string]*ecdsa.PrivateKey{}
	peers = NewPeers()
	for i := 0; i < N; i++ {
		key, _ := crypto.GenerateECDSAKey()
		netAddr := fmt.Sprintf("addr%d", i)
		keys[netAddr] = key
		peer := &Peer{
			NetAddr: netAddr,
			PubKey:  crypto.FromECDSAPub(&key.PublicKey),
		}
		peers.AddPeer(peer)
	}

	origin := peers.ToPeerSlice()

	err = store.Write(origin)
	assert.NoError(err)

	// Try a read, should find 3 peers
	peers, err = store.Read()
	assert.NoError(err)
	assert.Equal(N, peers.Len())

	restored := peers.ToPeerSlice()

	for i := 0; i < N; i++ {
		assert.EqualValues(origin[i].NetAddr, restored[i].NetAddr)
		assert.EqualValues(origin[i].PubKey, restored[i].PubKey)
		assert.EqualValues(&keys[origin[i].NetAddr].PublicKey, crypto.ToECDSAPub(restored[i].PubKey))
	}
}
