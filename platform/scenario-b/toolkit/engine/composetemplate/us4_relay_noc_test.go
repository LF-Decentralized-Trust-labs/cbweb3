// SPDX-License-Identifier: Apache-2.0

package composetemplate

import (
	"strings"
	"testing"
)

// US4: relay and noc templates validate and carry no fixed spoke identifier.
func TestUS4RelayAndNoc(t *testing.T) {
	for _, name := range []string{"relay", "noc-stack", "noc-agent"} {
		t.Run(name, func(t *testing.T) {
			tpl, env := loadTemplateAndEnv(t, name)
			if r := Validate(tpl, env); !r.OK {
				t.Fatalf("%s should validate: %+v", name, r.Errors)
			}
			for _, fixed := range []string{"spoke-a", "spoke-b"} {
				if strings.Contains(tpl.Raw, fixed) {
					t.Fatalf("%s must not hardcode %q", name, fixed)
				}
			}
		})
	}
}
