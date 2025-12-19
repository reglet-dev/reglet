// Package container provides dependency injection for the application.
package container

import (
	"log/slog"

	"github.com/whiskeyjimbo/reglet/internal/application/ports"
	"github.com/whiskeyjimbo/reglet/internal/application/services"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/adapters"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/redaction"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/system"
)

// Container holds all application dependencies.
type Container struct {
	// Ports (adapters)
	profileLoader    ports.ProfileLoader
	profileValidator ports.ProfileValidator
	systemConfig     ports.SystemConfigProvider
	pluginResolver   ports.PluginDirectoryResolver
	engineFactory    ports.EngineFactory

	// Use cases
	checkProfileUseCase *services.CheckProfileUseCase

	// Configuration
	trustPlugins bool
	systemCfg    *system.Config
	logger       *slog.Logger
}

// Options configure the container.
type Options struct {
	TrustPlugins     bool
	SecurityLevel    string // Security level: strict, standard, permissive (overrides config file)
	SystemConfigPath string
	Logger           *slog.Logger
}

// New creates a new dependency injection container.
func New(opts Options) (*Container, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	// Initialize adapters
	profileLoader := adapters.NewProfileLoaderAdapter()
	profileValidator := adapters.NewProfileValidatorAdapter()
	systemConfigAdapter := adapters.NewSystemConfigAdapter()
	pluginResolver := adapters.NewPluginDirectoryAdapter()

	// Load system config
	systemCfg, err := systemConfigAdapter.LoadConfig(nil, opts.SystemConfigPath)
	if err != nil {
		opts.Logger.Debug("failed to load system config, using defaults", "error", err)
		systemCfg = &system.Config{} // Use defaults
	}

	// Initialize redactor
	redactor, err := redaction.New(redaction.Config{
		Patterns: systemCfg.Redaction.Patterns,
		Paths:    systemCfg.Redaction.Paths,
		HashMode: systemCfg.Redaction.HashMode.Enabled,
		Salt:     systemCfg.Redaction.HashMode.Salt,
	})
	if err != nil {
		return nil, err
	}

	// Create engine factory
	engineFactory := adapters.NewEngineFactoryAdapter(redactor, systemCfg.WasmMemoryLimitMB)

	// Determine security level (command-line flag takes precedence over config file)
	securityLevel := opts.SecurityLevel
	if securityLevel == "" {
		// Use config file setting, or default to "standard"
		securityLevel = string(systemCfg.Security.GetSecurityLevel())
	}

	// Create capability orchestrator
	capOrchestrator := services.NewCapabilityOrchestratorWithSecurity(opts.TrustPlugins, securityLevel)

	// Wire up use case
	checkProfileUseCase := services.NewCheckProfileUseCase(
		profileLoader,
		profileValidator,
		systemConfigAdapter,
		pluginResolver,
		capOrchestrator,
		engineFactory,
		opts.Logger,
	)

	return &Container{
		profileLoader:       profileLoader,
		profileValidator:    profileValidator,
		systemConfig:        systemConfigAdapter,
		pluginResolver:      pluginResolver,
		engineFactory:       engineFactory,
		checkProfileUseCase: checkProfileUseCase,
		trustPlugins:        opts.TrustPlugins,
		systemCfg:           systemCfg,
		logger:              opts.Logger,
	}, nil
}

// CheckProfileUseCase returns the check profile use case.
func (c *Container) CheckProfileUseCase() *services.CheckProfileUseCase {
	return c.checkProfileUseCase
}

// ProfileLoader returns the profile loader port.
func (c *Container) ProfileLoader() ports.ProfileLoader {
	return c.profileLoader
}

// ProfileValidator returns the profile validator port.
func (c *Container) ProfileValidator() ports.ProfileValidator {
	return c.profileValidator
}

// SystemConfig returns the system configuration.
func (c *Container) SystemConfig() *system.Config {
	return c.systemCfg
}

// Logger returns the configured logger.
func (c *Container) Logger() *slog.Logger {
	return c.logger
}
