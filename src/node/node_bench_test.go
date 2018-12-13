package node

/*
go test -bench=BenchmarkNodes -run=^$
*/

import (
	"testing"

	"github.com/sirupsen/logrus"
)

const NODES = 5

func BenchmarkNodes(b *testing.B) {
	for n := 0; n < b.N; n++ {
		consensus(b, NODES)
	}
}

func consensus(b *testing.B, count int) {
	logger := logrus.New()
	nodes := NewNodeList(count, logger)
	b.ResetTimer()

	tx := []byte("INITIAL TRANSACTION")
	n0 := nodes.Values()[0]
	n0.PushTx(tx)
	nodes.WaitForConsensusTxsCount(1)

	b.StopTimer()
	nodes.Close()
}
