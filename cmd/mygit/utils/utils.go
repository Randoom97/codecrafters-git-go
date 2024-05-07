package utils

import (
	"fmt"
	"os"
)

func Error(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
