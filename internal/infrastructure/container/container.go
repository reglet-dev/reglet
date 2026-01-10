// Package container provides dependency injection for the application.
package container

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/application/services"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	domainservices "github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/infrastructure/adapters"
	"github.com/reglet-dev/reglet/internal/infrastructure/filesystem"
	"github.com/reglet-dev/reglet/internal/infrastructure/plugins"
	"github.com/reglet-dev/reglet/internal/infrastructure/redaction"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

// Container holds all application dependencies.
type Container struct {
	profileLoader       ports.ProfileLoader
	profileValidator    ports.ProfileValidator
	systemConfig        ports.SystemConfigProvider
	pluginResolver      ports.PluginDirectoryResolver
	engineFactory       ports.EngineFactory
	checkProfileUseCase *services.CheckProfileUseCase
	systemCfg           *system.Config
	logger              *slog.Logger
	trustPlugins        bool
}

// Options configure the container.
type Options struct {
	Logger           *slog.Logger
	SecurityLevel    string
	SystemConfigPath string
	TrustPlugins     bool
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
	systemCfg, err := systemConfigAdapter.LoadConfig(context.TODO(), opts.SystemConfigPath)
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

	// Create capability registry and register default extractors (OCP)
	capRegistry := capabilities.NewRegistry()
	plugins.RegisterDefaultExtractors(capRegistry)

	// Resolve config path for capability orchestrator
	// This follows 12-Factor App principles by passing config from cmd layer
	configPath := opts.SystemConfigPath
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configPath = filepath.Join(homeDir, ".reglet", "config.yaml")
		}
	}

	// Create plugin runtime factory (decouples application from wasm infrastructure)
	runtimeFactory := adapters.NewPluginRuntimeFactoryAdapter(redactor)

	// Create capability analyzer (domain service)
	capAnalyzer := domainservices.NewCapabilityAnalyzer(capRegistry)

	// Create capability gatekeeper (application service)
	capGatekeeper := services.NewCapabilityGatekeeper(configPath, securityLevel)

	// Create capability orchestrator with all dependencies injected
	// This makes the full dependency graph visible at the composition root
	capOrchestrator := services.NewCapabilityOrchestratorWithDeps(
		capAnalyzer,
		capGatekeeper,
		runtimeFactory,
		opts.TrustPlugins,
	)

	// Create lockfile infrastructure
	lockfileRepo := filesystem.NewFileLockfileRepository()
	versionResolver := plugins.NewSemverResolver()
	// TODO: Add real digester when available
	var pluginDigester ports.PluginDigester

	// Create application services
	lockfileService := services.NewLockfileService(lockfileRepo, versionResolver, pluginDigester)

	// Create domain services
	profileCompiler := domainservices.NewProfileCompiler()

	// Wire up use case
	checkProfileUseCase := services.NewCheckProfileUseCase(
		profileLoader,
		profileCompiler,
		profileValidator,
		systemConfigAdapter,
		pluginResolver,
		capOrchestrator,
		lockfileService,
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
