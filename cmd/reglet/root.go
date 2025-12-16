package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd is the application entry point.
var rootCmd = &cobra.Command{
	Use:   "reglet",
	Short: "Compliance and infrastructure validation platform",
	Long: `Reglet is a compliance and infrastructure validation platform built with 
WebAssembly extensibility and OSCAL-compliant output. It allows engineering 
teams to define policy as code, execute validation logic in sandboxed 
environments, and generate standardized audit artifacts.`,
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		setupLogging()
	},
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.reglet.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}

// initConfig loads configuration from the config file and environment.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Error("failed to find home directory", "error", err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".reglet")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		slog.Debug("using config file", "file", viper.ConfigFileUsed())
	}
}

func setupLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	// Using TextHandler for CLI friendliness
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}
