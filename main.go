package main

import (
	"os"
	"os/signal"

	"github.com/benleb/automoli-go/cmd"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/google/gops/agent"
	"github.com/muesli/termenv"
)

// var version = "dev"

func main() {
	if err := agent.Listen(agent.Options{
		Addr: "0.0.0.0:56765",
	}); err != nil {
		log.Fatal(err)
	}
	// cmd.AppVersion = version

	// cmd.BuildDate = buildDate
	// cmd.BuiltBy = builtBy

	lipgloss.SetColorProfile(termenv.TrueColor)

	// signal handler channel
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		sig := <-c

		// ctrl+c handler
		log.Debugf("Got %s signal. aborting...\n", sig)

		// cmd.GracefulShutdown()

		os.Exit(0)
	}()

	cmd.Execute()
}
