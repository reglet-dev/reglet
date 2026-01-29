package main

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
)

func init() {
	plugin.Register(&filePlugin{})
}

func main() {}
