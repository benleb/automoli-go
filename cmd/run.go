package cmd

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/benleb/automoli-go/internal/automoli"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
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
		var logLevel log.Level

		switch {
		case viper.GetBool("automoli.debug"):
			logLevel = log.DebugLevel

		case viper.GetBool("automoli.verbose"):
			logLevel = log.InfoLevel

		default:
			logLevel = log.WarnLevel
		}

		models.Printer = log.NewWithOptions(os.Stdout, log.Options{
			// ReportTimestamp: true,
			ReportTimestamp: false,
			TimeFormat:      " " + "15:04:05",
			ReportCaller:    logLevel < log.InfoLevel,
			Level:           logLevel,
		})

		// run automoli
		automoli.New()

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

	viper.SetDefault("homeassistant.lastMessageReceived.checkEvery", 7*time.Second)
	viper.SetDefault("homeassistant.lastMessageReceived.maxAge", 13*time.Second)
}
