package signature

import (
	"fmt"
	"math/big"
)

type Signature struct {
	R *big.Int
	S *big.Int
}

func (s *Signature) String() string {
	return fmt.Sprintf("%064x%064x", s.R, s.S)
}

func (s *Signature) Bytes() []byte {
	return s.R.Bytes()
}

func (s *Signature) Equals(other *Signature) bool {
	return s.R.Cmp(other.R) == 0 && s.S.Cmp(other.S) == 0
}
