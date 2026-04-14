package main

import (
	"fmt"
	"os"

	"gitoken/internal/gitoken"
)

func main() {
	if err := gitoken.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
