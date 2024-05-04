package utils

import (
	"fmt"
	"github.com/MikhUd/blockchain/pkg/consts"
)

func Print(color, text string) {
	fmt.Println(fmt.Sprintf("%s\n%s\n%s", color, text, consts.Reset))
}
