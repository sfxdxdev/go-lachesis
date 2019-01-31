package poset

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Fantom-foundation/go-lachesis/src/common"
	"github.com/Fantom-foundation/go-lachesis/src/peers"
)

func TestParticipantEventsCache(t *testing.T) {
	size := 10
	testSize := int64(25)
	participants := peers.NewPeersFromSlice([]*peers.Peer{
		peers.NewPeer(common.FromHex("0xaa"), ""),
		peers.NewPeer(common.FromHex("0xbb"), ""),
		peers.NewPeer(common.FromHex("0xcc"), ""),
	})

	pec := NewParticipantEventsCache(size, participants)

	items := make(map[common.Address]EventHashes)
	for id := range participants.ByID {
		items[id] = EventHashes{}
	}

	for i := int64(0); i < testSize; i++ {
		for id := range participants.ByID {
			item := fakeEventHash(fmt.Sprintf("%s%d", id.String(), i))

			pec.Set(id, item, i)

			pitems := items[id]
			pitems = append(pitems, item)
			items[id] = pitems
		}
	}

	// GET ITEM ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	for id := range participants.ByID {

		index1 := int64(9)
		_, err := pec.GetItem(id, index1)
		if err == nil || !common.Is(err, common.TooLate) {
			t.Fatalf("Expected ErrTooLate")
		}

		index2 := int64(15)
		expected2 := items[id][index2]
		actual2, err := pec.GetItem(id, index2)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected2, actual2) {
			t.Fatalf("expected and cached not equal")
		}

		index3 := int64(27)
		actual3, err := pec.Get(id, index3)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(EventHashes{}, actual3) {
			t.Fatalf("expected and cached not equal")
		}
	}

	//KNOWN ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	known := pec.Known()
	for p, k := range known {
		expectedLastIndex := testSize - 1
		if k != expectedLastIndex {
			t.Errorf("Known[%d] should be %d, not %d", p, expectedLastIndex, k)
		}
	}

	//GET ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	for id := range participants.ByID {
		if _, err := pec.Get(id, 0); err != nil && !common.Is(err, common.TooLate) {
			t.Fatalf("Skipping 0 elements should return ErrTooLate")
		}

		skipIndex := int64(9)
		expected := items[id][skipIndex+1:]
		cached, err := pec.Get(id, skipIndex)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected, cached) {
			t.Fatalf("expected and cached not equal")
		}

		skipIndex2 := int64(15)
		expected2 := items[id][skipIndex2+1:]
		cached2, err := pec.Get(id, skipIndex2)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected2, cached2) {
			t.Fatalf("expected and cached not equal")
		}

		skipIndex3 := int64(27)
		cached3, err := pec.Get(id, skipIndex3)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(EventHashes{}, cached3) {
			t.Fatalf("expected and cached not equal")
		}
	}
}

func TestParticipantEventsCacheEdge(t *testing.T) {
	size := 10
	testSize := int64(11)
	participants := peers.NewPeersFromSlice([]*peers.Peer{
		peers.NewPeer(common.FromHex("0xaa"), ""),
		peers.NewPeer(common.FromHex("0xbb"), ""),
		peers.NewPeer(common.FromHex("0xcc"), ""),
	})

	pec := NewParticipantEventsCache(size, participants)

	items := make(map[common.Address]EventHashes)
	for id := range participants.ByID {
		items[id] = EventHashes{}
	}

	for i := int64(0); i < testSize; i++ {
		for id := range participants.ByID {
			item := fakeEventHash(fmt.Sprintf("%s%d", id, i))

			pec.Set(id, item, i)

			pitems := items[id]
			pitems = append(pitems, item)
			items[id] = pitems
		}
	}

	for id := range participants.ByID {
		expected := items[id][size:]
		cached, err := pec.Get(id, int64(size-1))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected, cached) {
			t.Fatalf("expected (%#v) and cached (%#v) not equal", expected, cached)
		}
	}
}
