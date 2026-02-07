package entities

import (
	hostEntities "github.com/reglet-dev/reglet-host-sdk/plugin/entities"
)

// Forward plugin types to host-sdk.
// These aliases allow existing reglet code to continue using entities.Plugin etc.
// without updating all import paths at once. Remove once all callers are migrated.
type (
	Plugin              = hostEntities.Plugin
	PluginSpec          = hostEntities.PluginSpec
	PluginRegistry      = hostEntities.PluginRegistry
	Lockfile            = hostEntities.Lockfile
	PluginLock          = hostEntities.PluginLock
	ProfileLock         = hostEntities.ProfileLock
	IntegrityError      = hostEntities.IntegrityError
	PluginNotFoundError = hostEntities.PluginNotFoundError
)

var (
	NewPlugin                       = hostEntities.NewPlugin
	NewPluginRegistry               = hostEntities.NewPluginRegistry
	NewLockfile                     = hostEntities.NewLockfile
	ParsePluginDeclaration          = hostEntities.ParsePluginDeclaration
	ParsePluginDeclarationWithAlias = hostEntities.ParsePluginDeclarationWithAlias
	ErrPluginNotFound               = hostEntities.ErrPluginNotFound
	ErrIntegrityCheckFailed         = hostEntities.ErrIntegrityCheckFailed
)
