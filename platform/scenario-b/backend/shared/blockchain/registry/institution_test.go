// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

// The whole point of the institution id is that every wallet of one institution resolves to
// the same value. If this stops holding, the AMM resume quorum silently degrades from
// "two institutions" to "two keys" — the defect R2-H-4 exists to close.
func TestInstitutionIDForParticipant_SiblingWalletsAgree(t *testing.T) {
	first := InstitutionIDForParticipant("central-bank-brazil", "Banco Central do Brasil")
	second := InstitutionIDForParticipant("central-bank-brazil", "BCB (governance key 2)")
	if first != second {
		t.Fatal("two wallets of one institution must share an institutionId regardless of display name")
	}
}

func TestInstitutionIDForParticipant_DistinctInstitutionsDiffer(t *testing.T) {
	brazil := InstitutionIDForParticipant("central-bank-brazil", "Banco Central do Brasil")
	argentina := InstitutionIDForParticipant("central-bank-argentina", "BCRA")
	if brazil == argentina {
		t.Fatal("distinct institutions must not collide — the quorum would count them as one")
	}
}

// bankCode wins over name, so a caller cannot split one institution in two by spelling the
// name differently on a second registration.
func TestInstitutionIDForParticipant_BankCodeTakesPrecedence(t *testing.T) {
	if InstitutionIDForParticipant("bank-itau", "Itau") != InstitutionIDFromString("bank-itau") {
		t.Fatal("bankCode must be the source when present")
	}
	if InstitutionIDForParticipant("  ", "Itau") != InstitutionIDFromString("Itau") {
		t.Fatal("a blank bankCode must fall back to the name")
	}
}

func TestInstitutionIDFromString_NeverZero(t *testing.T) {
	// A zero id is rejected on-chain, so the derivation must never produce one for real input.
	if InstitutionIDFromString("central-bank-brazil") == ([32]byte{}) {
		t.Fatal("derivation produced the zero id, which the registry refuses to store")
	}
}

// InstitutionCodeFromEnv reports whether the code it found is unique per entity. This matters
// in Scenario B specifically: BANK_CODE is the entity ROLE (apply.go derives it from
// spec.topology.role), so every central bank carries "central-bank". Hashing that would make
// all central banks one institution, and a 2-of-N resume could never be met.
func TestInstitutionCodeFromEnv_FlagsTheRoleShapedFallback(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	code, unique := InstitutionCodeFromEnv(env(map[string]string{
		"INSTITUTION_CODE": "central-bank-brazil",
		"BANK_CODE":        "central-bank",
	}))
	if code != "central-bank-brazil" || !unique {
		t.Fatalf("INSTITUTION_CODE must win and be reported unique, got %q unique=%v", code, unique)
	}

	code, unique = InstitutionCodeFromEnv(env(map[string]string{
		"GOVERNANCE_BANK_CODE": "gov-brazil",
		"BANK_CODE":            "central-bank",
	}))
	if code != "gov-brazil" || !unique {
		t.Fatalf("GOVERNANCE_BANK_CODE must be the second source, got %q unique=%v", code, unique)
	}

	code, unique = InstitutionCodeFromEnv(env(map[string]string{"BANK_CODE": "central-bank"}))
	if code != "central-bank" {
		t.Fatalf("BANK_CODE is the last resort, got %q", code)
	}
	if unique {
		t.Fatal("BANK_CODE is the entity role and must NOT be reported as unique per entity — " +
			"callers rely on this flag to warn before registering a colliding institution")
	}

	// Two central banks configured the way the compose template configures them today land on
	// the same code. The flag is what makes that visible instead of silent.
	brazil, bOK := InstitutionCodeFromEnv(env(map[string]string{"BANK_CODE": "central-bank"}))
	argentina, aOK := InstitutionCodeFromEnv(env(map[string]string{"BANK_CODE": "central-bank"}))
	if brazil != argentina || bOK || aOK {
		t.Fatal("the role-shaped fallback must collide AND be reported as non-unique")
	}
}
