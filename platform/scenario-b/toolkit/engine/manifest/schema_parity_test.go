// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"os"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// schemaPath is the published JSON-Schema, relative to this test's package dir.
const schemaPath = "../../../provisioning/schema/v1/participant-deployment.schema.yaml"

// TestSchemaParity is the SC-004 parity check. Rather than run a runtime
// JSON-Schema library (which the toolkit deliberately avoids — see research.md
// R1), it loads the published schema and asserts its STRUCTURAL intent matches
// the Go validator's source-of-truth tables: the enum sets for mode/role/
// environment and the per-mode required/forbidden matrix. If the two ever drift
// this test fails, guaranteeing "schema and Go agree on structural rules".
func TestSchemaParity(t *testing.T) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	specProps := mapAt(t, root, "properties", "spec", "properties")

	// enum: mode
	assertEnum(t, mapAt(t, specProps, "mode"), Modes, "spec.mode")
	// enum: role
	role := mapAt(t, specProps, "topology", "properties", "role")
	assertEnum(t, role, Roles, "spec.topology.role")
	// enum: environment (Go accepts only "local")
	assertEnum(t, mapAt(t, specProps, "environment"), []string{"local"}, "spec.environment")

	// const fields
	assertConst(t, mapAt(t, root, "properties", "apiVersion"), wantAPIVersion, "apiVersion")
	assertConst(t, mapAt(t, root, "properties", "kind"), wantKind, "kind")
	assertConst(t, mapAt(t, specProps, "scenario"), wantScenario, "spec.scenario")

	// per-mode required / forbidden matrix from spec.allOf[].
	spec := mapAt(t, root, "properties", "spec")
	allOf, ok := spec["allOf"].([]any)
	if !ok {
		t.Fatal("spec.allOf missing or not a list")
	}

	gotRequired := map[string][]string{}
	gotForbidden := map[string][]string{}
	for _, item := range allOf {
		clause, _ := item.(map[string]any)
		mode := constString(mapAtOpt(clause, "if", "properties", "mode"))
		if mode == "" {
			continue
		}
		then, _ := clause["then"].(map[string]any)
		if then == nil {
			continue
		}
		gotRequired[mode] = stringList(then["required"])
		if not, ok := then["not"].(map[string]any); ok {
			if anyOf, ok := not["anyOf"].([]any); ok {
				var forbidden []string
				for _, a := range anyOf {
					am, _ := a.(map[string]any)
					forbidden = append(forbidden, stringList(am["required"])...)
				}
				gotForbidden[mode] = forbidden
			}
		}
	}

	for mode, want := range RequiredByMode {
		if !sameSet(gotRequired[mode], want) {
			t.Errorf("mode %s required mismatch: schema=%v go=%v", mode, gotRequired[mode], want)
		}
	}
	for mode, want := range ForbiddenByMode {
		if !sameSet(gotForbidden[mode], want) {
			t.Errorf("mode %s forbidden mismatch: schema=%v go=%v", mode, gotForbidden[mode], want)
		}
	}
}

// TestSchemaParityOnFixtures asserts the fixtures the Go validator accepts also
// satisfy the schema's declared per-mode required/forbidden fields (a
// lightweight parity check over the concrete examples).
func TestSchemaParityOnFixtures(t *testing.T) {
	for _, name := range []string{"found-hub.yaml", "found-spoke.yaml", "join.yaml", "observe.yaml"} {
		pd := mustLoad(t, name)
		if !Validate(pd).Valid() {
			t.Fatalf("%s: expected Go-valid fixture", name)
		}
		for _, f := range RequiredByMode[pd.Spec.Mode] {
			if !specFieldPresent(pd, f) {
				t.Errorf("%s: schema-required field %q absent from a Go-valid fixture", name, f)
			}
		}
		for _, f := range ForbiddenByMode[pd.Spec.Mode] {
			if specFieldPresent(pd, f) {
				t.Errorf("%s: schema-forbidden field %q present in a Go-valid fixture", name, f)
			}
		}
	}
}

// ---- helpers ----

func mapAt(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			t.Fatalf("schema path missing at key %q", k)
		}
		cur = next
	}
	return cur
}

func mapAtOpt(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func assertEnum(t *testing.T, node map[string]any, want []string, label string) {
	t.Helper()
	got := stringList(node["enum"])
	if !sameSet(got, want) {
		t.Errorf("%s enum mismatch: schema=%v go=%v", label, got, want)
	}
}

func assertConst(t *testing.T, node map[string]any, want, label string) {
	t.Helper()
	if got := constString(node); got != want {
		t.Errorf("%s const mismatch: schema=%q go=%q", label, got, want)
	}
}

func constString(node map[string]any) string {
	if node == nil {
		return ""
	}
	s, _ := node["const"].(string)
	return s
}

func stringList(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
