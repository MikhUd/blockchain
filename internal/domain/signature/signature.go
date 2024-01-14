package signature

import (
	"fmt"
	"math/big"
)

type Signature struct {
	R *big.Int
	S *big.Int
}

func (signature *Signature) String() string {
	return fmt.Sprintf("%064x%064x", signature.R, signature.S)
}
