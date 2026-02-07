package values

import (
	hostValues "github.com/reglet-dev/reglet-host-sdk/plugin/values"
)

type (
	PluginReference = hostValues.PluginReference
	Digest          = hostValues.Digest
	PluginMetadata  = hostValues.PluginMetadata
	PluginName      = hostValues.PluginName
)

var (
	NewPluginReference   = hostValues.NewPluginReference
	ParsePluginReference = hostValues.ParsePluginReference
	NewDigest            = hostValues.NewDigest
	ParseDigest          = hostValues.ParseDigest
	ComputeDigestSHA256  = hostValues.ComputeDigestSHA256
	NewPluginMetadata    = hostValues.NewPluginMetadata
	NewPluginName        = hostValues.NewPluginName
	MustNewPluginName    = hostValues.MustNewPluginName
)
