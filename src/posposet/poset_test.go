package posposet

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
)

func TestPoset(t *testing.T) {
	nodes, events := inter.GenEventsByNode(5, 99, 3)

	posets := make([]*Poset, len(nodes))
	inputs := make([]*EventStore, len(nodes))
	for i := 0; i < len(nodes); i++ {
		posets[i], _, inputs[i] = FakePoset(nodes)
		posets[i].SetName(nodes[i].String())
		posets[i].store.SetName(nodes[i].String())
	}

	t.Run("Multiple start", func(t *testing.T) {
		posets[0].Stop()
		posets[0].Start()
		posets[0].Start()
	})

	t.Run("Push unordered events", func(t *testing.T) {
		// first all events from one node
		for n := 0; n < len(nodes); n++ {
			ee := events[nodes[n]]
			for _, e := range ee {
				inputs[n].SetEvent(e)
				posets[n].PushEventSync(e.Hash())
			}
		}
		// second all events from others
		for n := 0; n < len(nodes); n++ {
			ee := events[nodes[n]]
			for _, e := range ee {
				for i := 0; i < len(posets); i++ {
					if i != n {
						inputs[i].SetEvent(e)
						posets[i].PushEventSync(e.Hash())
					}
				}
			}
		}
	})

	t.Run("All events in Store", func(t *testing.T) {
		assertar := assert.New(t)
		for _, ee := range events {
			for _, e0 := range ee {
				frame := posets[0].store.GetEventFrame(e0.Hash())
				if !assertar.NotNil(frame, "Event is not in poset store") {
					return
				}
			}
		}
	})

	t.Run("Check consensus", func(t *testing.T) {
		assertar := assert.New(t)
		for i := 0; i < len(posets)-1; i++ {
			p0 := posets[i]
			st := p0.store.GetState()
			t.Logf("poset%d: frame %d, block %d", i, st.LastFinishedFrameN, st.LastBlockN)
			for j := i + 1; j < len(posets); j++ {
				p1 := posets[j]

				// compare blockchain
				if !assertar.Equal(p0.state.LastBlockN, p1.state.LastBlockN, "blocks count") {
					return
				}
				for b := uint64(1); b <= p0.state.LastBlockN; b++ {
					if !assertar.Equal(p0.store.GetBlock(b), p1.store.GetBlock(b), "block") {
						return
					}
				}
			}
		}
	})

	t.Run("Multiple stop", func(t *testing.T) {
		posets[0].Stop()
		posets[0].Stop()
	})
}

/*
 * Poset's test methods:
 */

// PushEventSync takes event into processing.
// It's a sync version of Poset.PushEvent().
func (p *Poset) PushEventSync(e hash.Event) {
	event := p.input.GetEvent(e)
	p.onNewEvent(event)
}
