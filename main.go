package main

import (
	"os"
	"os/signal"

	"github.com/benleb/automoli-go/cmd"
	"github.com/benleb/automoli-go/internal/automoli"
	"github.com/charmbracelet/log"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {

	automoli.AppVersion = version
	automoli.CommitDate = buildDate
	automoli.Commit = commit

	// signal handler channel
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		sig := <-c

		// ctrl+c handler
		log.Debugf("Got %s signal. aborting...\n", sig)

		os.Exit(0)
	}()

	cmd.Execute()
}
