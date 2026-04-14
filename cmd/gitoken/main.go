package main

import (
	"fmt"
	"os"

	"github.com/849261680/token-heatmap/internal/gitoken"
)

func main() {
	if err := gitoken.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
