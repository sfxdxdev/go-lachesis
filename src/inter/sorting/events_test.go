package sorting

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"github.com/Fantom-foundation/go-lachesis/src/hash"
	"github.com/Fantom-foundation/go-lachesis/src/inter"
)

func TestNewOrder(t *testing.T) {
	type args struct {
		source Source
		log    *logrus.Logger
		async  []bool
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	source, logger, _ := FakeOrder(ctrl, false)

	incomplete := make(map[hash.Event]*internalEvent)
	tests := []struct {
		name    string
		args    args
		want    *Ordering
		wantErr bool
	}{
		{name: "working 1",
			args:    args{source: source, log: logger},
			want:    &Ordering{source, logger, false, incomplete},
			wantErr: false,
		},
		{name: "working 2",
			args:    args{source: source, log: logger, async: []bool{false}},
			want:    &Ordering{source, logger, false, incomplete},
			wantErr: false,
		},
		{name: "not working 1",
			args:    args{source: source, log: logger, async: []bool{true}},
			want:    &Ordering{source, logger, false, incomplete},
			wantErr: true,
		},
		{name: "not working 2",
			args:    args{source: source, log: logger},
			want:    &Ordering{source, logger, true, incomplete},
			wantErr: true,
		},
		{name: "not working 3",
			args:    args{source: source, log: logger, async: []bool{false}},
			want:    &Ordering{source, logger, true, incomplete},
			wantErr: true,
		},
		{name: "not working 4",
			args:    args{source: nil, log: logger},
			want:    &Ordering{source, logger, true, incomplete},
			wantErr: true,
		},
		{name: "not working 5",
			args:    args{source: source, log: nil},
			want:    &Ordering{source, logger, true, incomplete},
			wantErr: true,
		},
		{name: "not working 6",
			args:    args{source: source, log: logger},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewOrder(tt.args.source, tt.args.log, tt.args.async...)
			equal := reflect.DeepEqual(got, tt.want)
			if !equal && !tt.wantErr || equal && tt.wantErr {
				t.Errorf("NewOrder() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("without async argument", func(t *testing.T) {
		got := NewOrder(source, logger)
		assert.False(t, got.async)
	})

	t.Run("call with async argument equal to true", func(t *testing.T) {
		got := NewOrder(source, logger, false)
		assert.False(t, got.async)
	})

	t.Run("call with async argument equal to false", func(t *testing.T) {
		got := NewOrder(source, logger, true)
		assert.True(t, got.async)
	})
}

func TestOrdering_NewEventConsumer(t *testing.T) {
	type args struct {
		e  *inter.Event
		cb []callbacks
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, _, order := FakeOrder(ctrl, true)

	event := inter.FakeFuzzingEvents()[0]

	internal := &internalEvent{
		Event:         *event,
		consensusTime: 0,
		parents:       make(map[hash.Event]*internalEvent, len(event.Parents)),
	}

	for pHash := range event.Parents {
		internal.parents[pHash] = nil
	}

	valid := &eventStream{
		event: internal,

		out:   make(chan *inter.Event),
		exit:  make(chan struct{}, 10),
		ready: make(chan struct{}, 1),

		async: false,
	}

	tests := []struct {
		name    string
		o       *Ordering
		args    args
		want    *eventStream
		wantErr bool
	}{
		{name: "working 1",
			args:    args{e: event},
			o:       order,
			want:    &eventStream{valid.event, valid.out, valid.exit, valid.ready, false},
			wantErr: false,
		},
		{name: "not working 1",
			args:    args{e: event},
			o:       order,
			want:    &eventStream{valid.event, nil, nil, nil, false},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.o.NewEventConsumer(tt.args.e, tt.args.cb...)

			gotExitType := reflect.TypeOf(got.exit)
			var wantExitType reflect.Type
			if tt.want.exit != nil {
				wantExitType = reflect.TypeOf(tt.want.exit)
			}
			equalExit := reflect.DeepEqual(gotExitType, wantExitType)
			if !equalExit && !tt.wantErr || equalExit && tt.wantErr {
				t.Errorf("exit channel type = %v, want %v", gotExitType, wantExitType)
			}

			gotReady := reflect.TypeOf(got.ready)
			var wantReady reflect.Type
			if tt.want.ready != nil {
				wantReady = reflect.TypeOf(tt.want.ready)
			}
			equalReady := reflect.DeepEqual(gotReady, wantReady)
			if !equalReady && !tt.wantErr || equalReady && tt.wantErr {
				t.Errorf("ready channel type = %v, want %v", gotReady, wantReady)
			}

			gotOut := reflect.TypeOf(got.out)
			var wantOut reflect.Type
			if tt.want.out != nil {
				wantOut = reflect.TypeOf(tt.want.out)
			}
			equalOut := reflect.DeepEqual(gotOut, wantOut)
			if !equalOut && !tt.wantErr || equalOut && tt.wantErr {
				t.Errorf("exit channel type = %v, want %v", gotOut, wantOut)
			}
		})
	}
}

func Test_consumer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, _, order := FakeOrder(ctrl, true)
	events := inter.FakeFuzzingEvents()

	t.Run("consumer with streaming events", func(t *testing.T) {
		for _, ee := range events {
			ch := order.NewEventConsumer(ee)
			consumer(ch)
			assert.NotNil(t, ch)
		}
	})

	t.Run("", func(t *testing.T) {
		var ch *eventStream
		consumer(ch)
		assert.Nil(t, ch)
	})
}

func Test_worker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	source := NewMockSource(ctrl)
	events := inter.FakeFuzzingEvents()
	order := NewOrder(source, nil)
	logger, hook := test.NewNullLogger()

	tests := []struct {
		level logrus.Level
		msg   string
	}{
		{logrus.InfoLevel, "info text"},
		{logrus.DebugLevel, "debug text"},
		{logrus.WarnLevel, "warn text"},
	}

	t.Run("worker with channels", func(t *testing.T) {
		for _, ee := range events {
			index := rand.Intn(len(tests))
			level, msg := tests[index].level, tests[index].msg

			cb := func(event *inter.Event) {
				logger.Log(level, msg)
			}

			ch := order.NewEventConsumer(ee)
			go worker(ch, cb)
			ch.out <- &inter.Event{}
			<-ch.ready
			ch.exit <- struct{}{}
			if hook.LastEntry() != nil {
				assert.Equal(t, level, hook.LastEntry().Level)
				assert.Equal(t, msg, hook.LastEntry().Message)
			}
			hook.Reset()
		}
	})
}

func TestOrdering_NewEventOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	source := NewMockSource(ctrl)
	events := inter.FakeFuzzingEvents()
	logger, hook := test.NewNullLogger()
	order := NewOrder(source, logger)

	t.Run("", func(t *testing.T) {
		vf := func(event hash.Event) *uint64 {
			var value uint64 = 1
			return &value
		}
		for _, ee := range events {
			stream := order.NewEventConsumer(ee)
			order.NewEventOrder(stream, vf)

			if hook.LastEntry() != nil {
				assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
				assert.Equal(t, "event had received already", strings.ToLower(hook.LastEntry().Message))
			}
			hook.Reset()
		}
	})
}

func TestOrdering_newEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	source := NewMockSource(ctrl)
	logger, _ := test.NewNullLogger()
	order := NewOrder(source, logger)

	nodes, inters := inter.GenEventsByNode(5, 99, 3)
	nodesEvents := make(map[hash.Peer][]*internalEvent, len(inters))
	for h, from := range inters {
		to := make([]*internalEvent, len(from))
		for i, e := range from {
			to[i] = &internalEvent{Event: *e}
		}
		nodesEvents[h] = to
	}

	t.Run("", func(t *testing.T) {
		vf := func(event hash.Event) *uint64 {
			return nil
		}
		for n := 0; n < len(nodes); n++ {
			events := nodesEvents[nodes[n]]
			for _, e := range events {
				source.
					EXPECT().
					SetEvent(&e.Event).
					AnyTimes()
				for pHash := range e.Parents {
					source.
						EXPECT().
						GetEvent(pHash).
						Return(&e.Event).
						AnyTimes()
				}

				eventStream := order.NewEventConsumer(&e.Event)
				order.newEvent(eventStream, vf)
			}
		}
	})

	t.Run("", func(t *testing.T) {
		for n := 0; n < len(nodes); n++ {
			events := nodesEvents[nodes[n]]
			for _, e := range events {
				source.
					EXPECT().
					SetEvent(&e.Event).
					AnyTimes()
				for pHash := range e.Parents {
					source.
						EXPECT().
						GetEvent(pHash).
						Return(nil).
						AnyTimes()
				}

				eventStream := order.NewEventConsumer(&e.Event)
				order.newEvent(eventStream)
			}
		}
	})
}

func FakeOrder(ctrl *gomock.Controller, withOrder bool) (*MockSource, *logrus.Logger, *Ordering) {
	source := NewMockSource(ctrl)
	logger, _ := test.NewNullLogger()
	order := &Ordering{}
	if withOrder {
		order = NewOrder(source, logger)
	}
	return source, logger, order
}
