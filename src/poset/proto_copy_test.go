package poset

import (
	"testing"

	"github.com/golang/protobuf/proto"
)

func Test(t *testing.T) {
	src := &Block{
		Body:       &BlockBody{},
		Signatures: make(map[string]string),
		Hash:       []byte{11, 22, 33, 44, 55, 66},
		Hex:        "1231231",
	}

	dst := protoCopy(src).(proto.Message)

	if !proto.Equal(src, dst) {
		t.Fatalf("\n%+v\n!=\n%+v", src, dst)
	}

	if src.Signatures != nil && dst.(*Block).Signatures == nil {
		// NOTE: be careful, default fields values become nil after proto.Clone()
	}
}
