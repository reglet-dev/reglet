package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	logLevel string
	quiet    bool
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.reglet/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress all log output (equivalent to --log-level=error)")
}

// initConfig loads configuration from the config file and environment.
func initConfig() {
	if cfgFile != "" {
		// User explicitly specified a config file - it must exist
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			slog.Error("failed to read specified config file", "file", cfgFile, "error", err)
			os.Exit(1)
		}
		slog.Debug("using config file", "file", viper.ConfigFileUsed())
		return
	}

	// Default config path - optional, don't fail if missing
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to find home directory", "error", err)
		os.Exit(1)
	}

	viper.AddConfigPath(home + "/.reglet")
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		slog.Debug("using config file", "file", viper.ConfigFileUsed())
	}
	// Silently continue if default config doesn't exist
}

func setupLogging() {
	level := parseLogLevel(logLevel)

	// --quiet flag overrides --log-level to suppress output
	if quiet {
		level = slog.LevelError + 1 // Above error = effectively silent
	}

	// Using TextHandler for CLI friendliness
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
