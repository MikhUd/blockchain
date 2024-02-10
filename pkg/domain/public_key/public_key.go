package public_key

import (
	"crypto/elliptic"
	"fmt"
	"math/big"
)

type PublicKey struct {
	elliptic.Curve
	X, Y *big.Int
}

func (pk *PublicKey) String() string {
	return fmt.Sprintf("%064x%064x", pk.X, pk.Y)
}
