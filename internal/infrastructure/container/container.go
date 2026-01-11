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
	infraconfig "github.com/reglet-dev/reglet/internal/infrastructure/config"
	"github.com/reglet-dev/reglet/internal/infrastructure/filesystem"
	"github.com/reglet-dev/reglet/internal/infrastructure/output"
	"github.com/reglet-dev/reglet/internal/infrastructure/plugins"
	embeddedplugin "github.com/reglet-dev/reglet/internal/infrastructure/plugins/embedded"
	ociplugin "github.com/reglet-dev/reglet/internal/infrastructure/plugins/oci"
	pluginrepo "github.com/reglet-dev/reglet/internal/infrastructure/plugins/repository"
	signingplugin "github.com/reglet-dev/reglet/internal/infrastructure/plugins/signing"
	"github.com/reglet-dev/reglet/internal/infrastructure/secrets"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
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
	pluginService       *services.PluginService
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

	// Initialize sensitive value provider first (needed for redactor and resolver)
	sensitiveProvider := sensitivedata.NewProvider()

	systemConfigAdapter := adapters.NewSystemConfigAdapter()
	systemCfg, err := systemConfigAdapter.LoadConfig(context.Background(), opts.SystemConfigPath)
	if err != nil {
		opts.Logger.Debug("failed to load system config, using defaults", "error", err)
		systemCfg = &system.Config{} // Use defaults
	}

	// Create resolver with config from system config
	secretResolver := secrets.NewResolver(&systemCfg.SensitiveData.Secrets, sensitiveProvider)

	// Initialize adapters
	profileLoader := adapters.NewProfileLoaderAdapter(secretResolver)
	profileValidator := adapters.NewProfileValidatorAdapter()
	pluginResolver := adapters.NewPluginDirectoryAdapter()

	// Initialize redactor with shared provider
	redactor, err := sensitivedata.NewWithProvider(sensitivedata.Config{
		Patterns: systemCfg.Redaction.Patterns,
		Paths:    systemCfg.Redaction.Paths,
		HashMode: systemCfg.Redaction.HashMode.Enabled,
		Salt:     systemCfg.Redaction.HashMode.Salt,
	}, sensitiveProvider)
	if err != nil {
		return nil, err
	}

	// Create and configure runtime config
	runtimeCfg := infraconfig.FromSystemConfig(systemCfg)
	runtimeCfg.ApplyDefaults()

	// Create engine factory
	engineFactory := adapters.NewEngineFactoryAdapter(redactor, runtimeCfg)

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

	// --- Plugin Management Wiring ---

	// 1. Auth Provider
	authProvider := ociplugin.NewEnvAuthProvider()

	// 2. Registry Adapter
	registryAdapter := ociplugin.NewOCIRegistryAdapter(authProvider)

	// 3. Plugin Repository (Plugin Cache)
	homeDir, _ := os.UserHomeDir() // Ignore error for now, worst case cache dir logic handles it or fails
	cacheDir := filepath.Join(homeDir, ".reglet", "plugins")
	pluginRepository, err := pluginrepo.NewFSPluginRepository(cacheDir)
	if err != nil {
		return nil, err
	}

	// 4. Integrity Verifier
	integrityVerifier := signingplugin.NewCosignVerifier(nil, nil)

	// 5. Embedded Source
	embeddedSource := embeddedplugin.NewEmbeddedSource()

	// 6. Domain Services
	// TODO: Get requireSigning from configuration
	integrityService := domainservices.NewIntegrityService(false)

	// 7. Resolvers (Chain of Responsibility)
	// Resolution Order: Embedded -> Cache -> Registry
	registryResolver := services.NewRegistryPluginResolver(
		registryAdapter,
		pluginRepository,
		opts.Logger,
	)

	cachedResolver := services.NewCachedPluginResolver(pluginRepository)
	cachedResolver.SetNext(registryResolver)

	embeddedResolver := services.NewEmbeddedPluginResolver(embeddedSource)
	embeddedResolver.SetNext(cachedResolver)

	// 8. Plugin Service (Application Service)
	pluginService := services.NewPluginService(
		embeddedResolver, // Head of the chain
		pluginRepository,
		registryAdapter,
		integrityVerifier,
		integrityService,
		opts.Logger,
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
		pluginService,
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
		pluginService:       pluginService,
		trustPlugins:        opts.TrustPlugins,
		systemCfg:           systemCfg,
		logger:              opts.Logger,
	}, nil
}

// CheckProfileUseCase returns the check profile use case.
func (c *Container) CheckProfileUseCase() *services.CheckProfileUseCase {
	return c.checkProfileUseCase
}

// PluginService returns the plugin service.
func (c *Container) PluginService() *services.PluginService {
	return c.pluginService
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

// OutputFormatterFactory returns the output formatter factory port.
func (c *Container) OutputFormatterFactory() ports.OutputFormatterFactory {
	return output.NewFormatterFactory()
}

// Logger returns the configured logger.
func (c *Container) Logger() *slog.Logger {
	return c.logger
}
