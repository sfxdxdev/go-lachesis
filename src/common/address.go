package common

import (
	"reflect"

	"github.com/Fantom-foundation/go-lachesis/src/common/hexutil"
)

var (
	addressT = reflect.TypeOf(Address{})
)

// Address represents the address of an PoS account.
type Address Hash

// Bytes gets the byte representation of the underlying hash.
func (a Address) Bytes() []byte { return Hash(a).Bytes() }

// String implements the stringer interface.
func (a Address) String() string { return Hash(a).String() }

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalText()
}

// UnmarshalText parses a hash in hex syntax.
func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

// UnmarshalJSON parses a hash in hex syntax.
func (a *Address) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(addressT, input, a[:])
}

// BytesToAddress sets b to address.
// If b is larger than len(h), b will be cropped from the right.
func BytesToAddress(b []byte) Address {
	return Address(BytesToHash(b))
}
