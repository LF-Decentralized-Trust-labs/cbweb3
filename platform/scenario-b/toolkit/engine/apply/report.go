package apply

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/orchestrator"
)

// Render serializes a Report in the requested format (json | yaml; default yaml).
func Render(r orchestrator.Report, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.MarshalIndent(r, "", "  ")
	case "yaml", "":
		return yaml.Marshal(r)
	default:
		return nil, fmt.Errorf("unsupported report format %q (want json|yaml)", format)
	}
}
