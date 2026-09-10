package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/kesonglab/video-interpolate/internal/cli"
	projlog "github.com/kesonglab/video-interpolate/internal/log"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "vif crashed: %v\n%s\n", r, debug.Stack())
			os.Exit(2)
		}
	}()
	if err := cli.Execute(); err != nil {
		projlog.New(os.Stderr, projlog.LevelError).Error(err)
		os.Exit(1)
	}
}
