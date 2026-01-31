package main

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"

	_ "github.com/reglet-dev/reglet/plugins/aws/services"
)

func main() {
	// Register the implementation
	plugin.Register(&awsPlugin{})
}
