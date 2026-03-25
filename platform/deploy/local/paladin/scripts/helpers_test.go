// Package scripts_test contains shared helpers used by the Paladin deploy and
// register test scripts. These helpers read Paladin K8s artifact YAML files to
// extract contract bytecode for deployment.
package scripts_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"gopkg.in/yaml.v3"
)

// artifactSpec mirrors the spec.bytecode and spec.linkReferencesJSON fields of
// a Paladin SmartContractDeployment artifact YAML.
type artifactSpec struct {
	Bytecode           string `yaml:"bytecode"`
	LinkReferencesJSON string `yaml:"linkReferencesJSON"`
}

type artifactDoc struct {
	Spec artifactSpec `yaml:"spec"`
}

type linkRef struct {
	Length int `json:"length"`
	Start  int `json:"start"`
}

// linkPlaceholderRE matches unlinked library placeholders like __$<hash>$__
var linkPlaceholderRE = regexp.MustCompile(`__\$[0-9a-f]+\$__`)

// readArtifact parses a Paladin artifact YAML and returns (bytecodeBytes, linkRefs).
// linkRefs maps libName → []linkRef.
func readArtifact(t *testing.T, path string) ([]byte, map[string][]linkRef) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readArtifact(%s): %v", path, err)
	}
	var doc artifactDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("readArtifact(%s) YAML: %v", path, err)
	}
	hexStr := strings.TrimPrefix(strings.TrimSpace(doc.Spec.Bytecode), "0x")
	// Replace unlinked library placeholders with zeros so hex.DecodeString succeeds.
	hexStr = linkPlaceholderRE.ReplaceAllStringFunc(hexStr, func(m string) string {
		return strings.Repeat("0", len(m))
	})
	bytecode, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("readArtifact(%s) hex: %v", path, err)
	}
	refs := map[string][]linkRef{}
	if doc.Spec.LinkReferencesJSON != "" {
		var fileLibMap map[string]map[string][]linkRef
		if err := json.Unmarshal([]byte(doc.Spec.LinkReferencesJSON), &fileLibMap); err != nil {
			t.Fatalf("readArtifact(%s) linkRefs: %v", path, err)
		}
		for _, libs := range fileLibMap {
			for libName, lrs := range libs {
				refs[libName] = append(refs[libName], lrs...)
			}
		}
	}
	return bytecode, refs
}

// linkBytecode patches library addresses into a copy of the bytecode.
// addrs maps libName → common.Address.
func linkBytecode(t *testing.T, bytecode []byte, refs map[string][]linkRef, addrs map[string]common.Address) []byte {
	t.Helper()
	result := make([]byte, len(bytecode))
	copy(result, bytecode)
	for libName, lrs := range refs {
		addr, ok := addrs[libName]
		if !ok {
			t.Fatalf("linkBytecode: no address for library %q", libName)
		}
		for _, ref := range lrs {
			copy(result[ref.Start:ref.Start+ref.Length], addr.Bytes())
		}
	}
	return result
}
