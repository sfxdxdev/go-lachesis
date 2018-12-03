package poset

import (
	"github.com/golang/protobuf/proto"
)

// protoCopy returns a deep copy of a proto.Message,
// but takes an interface{} as args for convenience.
// Be careful, default fields values become nil.
func protoCopy(src interface{}) interface{} {
	return proto.Clone(src.(proto.Message))
}
