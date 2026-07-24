package main

import (
	"fmt"
	"os"

	"d3-movie-downloader/internal/bot"
)

func main() {
	b := bot.NewBot()
	if err := b.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to start application: %v\n", err)
		os.Exit(1)
	}
}
