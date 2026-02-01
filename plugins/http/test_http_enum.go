//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"github.com/reglet-dev/reglet/plugins/http/core"
	_ "github.com/reglet-dev/reglet/plugins/http/services"
)

func main() {
	manifest := core.Plugin.Manifest()
	data, _ := json.MarshalIndent(manifest.ConfigSchema["properties"].(map[string]any)["method"], "", "  ")
	fmt.Println(string(data))
}
