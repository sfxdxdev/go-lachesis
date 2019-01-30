package peers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/Fantom-foundation/go-lachesis/src/common"
)

const (
	JsonFileName = "peers.json"
)

// JSONPeers is used to provide peer persistence on disk in the form
// of a JSON file. It implements the PeerStore interface.
type JSONPeers struct {
	l    sync.Mutex
	path string
}

type jsonPeer struct {
	NetAddr   string `json:"NetAddr,omitempty"`
	PubKeyHex string `json:"PubKeyHex,omitempty"`
}

// NewJSONPeers creates a new JSONPeers store.
func NewJSONPeers(base string) *JSONPeers {
	path := filepath.Join(base, JsonFileName)
	return &JSONPeers{
		path: path,
	}
}

// Read is the PeerStore interface implementation.
func (j *JSONPeers) Read() (*Peers, error) {
	j.l.Lock()
	defer j.l.Unlock()

	buf, err := j.readOrCreate()
	if err != nil {
		return nil, err
	}

	var data []*jsonPeer
	if len(buf) > 0 {
		dec := json.NewDecoder(bytes.NewReader(buf))
		if err := dec.Decode(&data); err != nil {
			return nil, err
		}
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("peers not found")
	}

	peers := make([]*Peer, 0, len(data))
	for _, d := range data {
		peers = append(peers, NewPeer(
			common.FromHex(d.PubKeyHex),
			d.NetAddr,
		))
	}

	return NewPeersFromSlice(peers), nil
}

// Write is the PeerStore interface implementation.
func (j *JSONPeers) Write(peers []*Peer) error {
	j.l.Lock()
	defer j.l.Unlock()

	data := make([]*jsonPeer, 0, len(peers))
	for _, p := range peers {
		data = append(data, &jsonPeer{
			NetAddr:   p.NetAddr,
			PubKeyHex: common.ToHex(p.PubKey),
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return err
	}

	// Write out as JSON
	return ioutil.WriteFile(j.path, buf.Bytes(), 0755)
}

// Reads the file or create empty.
func (j *JSONPeers) readOrCreate() ([]byte, error) {
	buf, err := ioutil.ReadFile(j.path)
	if err != nil {
		err = os.MkdirAll(filepath.Dir(j.path), 0750)
		if err != nil {
			return nil, err
		}
		f, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil {
			return nil, err
		}
		f.Close()
	}

	return buf, err
}
