// SPDX-License-Identifier: Apache-2.0

package composetemplate

import "testing"

// US2: state uses deterministic named volumes; only host bind allowed is pki/.

func TestUS2EntityBesuUsesNamedVolumes(t *testing.T) {
	tpl, env := loadTemplateAndEnv(t, "entity-besu")
	if r := Validate(tpl, env); !r.OK {
		t.Fatalf("entity-besu should satisfy named-volumes: %+v", r.Errors)
	}
}

func TestUS2RejectsHostBindForState(t *testing.T) {
	tpl := &Template{Raw: `services:
  besu:
    image: x
    volumes:
      - "/host/nodes/data:/opt/besu/data"
volumes: {}
`}
	r := Validate(tpl, map[string]string{})
	if r.OK || !hasRule(r, "named-volumes") {
		t.Fatalf("expected named-volumes rejection, got %+v", r.Errors)
	}
}

func TestUS2AllowsPkiHostBind(t *testing.T) {
	tpl := &Template{Raw: `services:
  besu:
    image: x
    volumes:
      - "/host/bank-a/pki:/opt/besu/pki"
volumes: {}
`}
	if r := Validate(tpl, map[string]string{}); !r.OK {
		t.Fatalf("pki host bind must be allowed, got %+v", r.Errors)
	}
}

func TestUS2RejectsUnnamedTopLevelVolume(t *testing.T) {
	tpl := &Template{Raw: `services:
  db:
    image: x
    volumes:
      - data:/var/lib
volumes:
  data: {}
`}
	r := Validate(tpl, map[string]string{})
	if r.OK || !hasRule(r, "named-volumes") {
		t.Fatalf("unnamed top-level volume should fail, got %+v", r.Errors)
	}
}
