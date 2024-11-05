package cmd

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/benleb/automoli-go/internal/automoli"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runCmd represents the run command.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: automoli.AppIcon + " run AutoMoLi",

	Run: func(_ *cobra.Command, _ []string) {
		// print header/logo
		randHeaderID, _ := rand.Int(rand.Reader, big.NewInt(int64(len(automoli.LogoHeader))))
		headerLogo := automoli.LogoHeader[randHeaderID.Int64()]

		fmt.Println(lipgloss.NewStyle().Padding(2, 4).Render(headerLogo))

		// general log settings & style
		lipgloss.SetColorProfile(termenv.TrueColor)
		log.SetColorProfile(termenv.TrueColor)

		var logLevel log.Level

		// set log level
		switch {
		case viper.GetBool("automoli.debug"):
			logLevel = log.DebugLevel

		case viper.GetBool("automoli.verbose"):
			logLevel = log.InfoLevel

		default:
			logLevel = log.WarnLevel
		}

		// report timestamps only when run in docker, otherwise we rely on systemd journald or similar
		reportTimestamp := runningInDocker()

		models.Printer = log.NewWithOptions(os.Stdout, log.Options{
			ReportTimestamp: reportTimestamp,
			TimeFormat:      " " + "15:04:05",
			ReportCaller:    logLevel < log.InfoLevel,
			Level:           logLevel,
		})

		// set color profile for loggers
		models.Printer.SetColorProfile(termenv.TrueColor)

		// run automoli
		if aml := automoli.New(); aml == nil {
			models.Printer.Error("failed to initialize AutoMoLi")

			os.Exit(1)
		}

		// loopy mcLoopface 😵‍💫
		select {}
	},
}

func init() { //nolint:gochecknoinits
	rootCmd.AddCommand(runCmd)

	// logging
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "show more output")
	_ = viper.BindPFlag("automoli.verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "show debug output")
	_ = viper.BindPFlag("automoli.debug", rootCmd.PersistentFlags().Lookup("debug"))

	// defaults
	viper.SetDefault("automoli.defaults.delay", 337*time.Second)
	viper.SetDefault("automoli.defaults.relax_after_turn_on", 1337*time.Millisecond)
	viper.SetDefault("automoli.defaults.transition", 2*time.Second)
	viper.SetDefault("automoli.defaults.flash", "")
	viper.SetDefault("automoli.defaults.stats_interval", "13m37s")

	// last event received watchdog
	viper.SetDefault("homeassistant.defaults.watchdog_max_age", 17*time.Second)
	viper.SetDefault("homeassistant.defaults.watchdog_check_every", 7*time.Second)
}

// // runningViaSystemd checks if the process is running via systemd.
// func runningViaSystemd() bool {
// 	return os.Getenv("INVOCATION_ID") != ""
// }

// runningInDocker checks if the process is running in a docker container.
func runningInDocker() bool {
	_, err := os.Stat("/.dockerenv")

	return !errors.Is(err, os.ErrNotExist)
}

// // runningInKubernetes checks if the process is running in a kubernetes cluster.
// func runningInKubernetes() bool {
// 	_, err := os.Stat("/var/run/secrets/kubernetes.io")

// 	return !errors.Is(err, os.ErrNotExist)
// }
