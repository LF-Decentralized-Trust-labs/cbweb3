// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "testing"

func TestEntityPorts_DerivedFromBesuRPC(t *testing.T) {
	p := entityPorts(8645) // CB Besu RPC
	cases := map[string]struct{ got, want int }{
		"APIGateway":        {p.APIGateway, 18645},
		"AuthGRPC":          {p.AuthGRPC, 19645},
		"ComplianceGRPC":    {p.ComplianceGRPC, 20645},
		"PaymentGRPC":       {p.PaymentGRPC, 21645},
		"Postgres":          {p.Postgres, 22645},
		"Redis":             {p.Redis, 23645},
		"Keycloak":          {p.Keycloak, 24645},
		"FrontendPrimary":   {p.FrontendPrimary, 25645},
		"FrontendSecondary": {p.FrontendSecondary, 26645},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d; want %d", name, c.got, c.want)
		}
	}
}

func TestEntityPorts_UniqueAcrossEntities(t *testing.T) {
	// Distinct Besu RPC ports (CB 8645, itau 8646, bradesco 8647) must yield
	// fully disjoint port sets — no collision on one host.
	seen := map[int]string{}
	for _, besu := range []int{8645, 8646, 8647, 8745, 8746} {
		p := entityPorts(besu)
		for _, port := range []int{p.APIGateway, p.AuthGRPC, p.ComplianceGRPC, p.PaymentGRPC,
			p.Postgres, p.Redis, p.Keycloak, p.FrontendPrimary, p.FrontendSecondary} {
			if owner, dup := seen[port]; dup {
				t.Errorf("port %d collides: besu=%d and %s", port, besu, owner)
			}
			seen[port] = "besu-" + itoaT(besu)
		}
	}
}

func TestEntityPorts_AvoidPaladinRange(t *testing.T) {
	// Paladin host ports are besuRPC+23000/+1/+2; the 034 offsets must not overlap.
	besu := 8645
	p := entityPorts(besu)
	paladin := map[int]bool{besu + 23000: true, besu + 23001: true, besu + 23002: true}
	for _, port := range []int{p.APIGateway, p.AuthGRPC, p.ComplianceGRPC, p.PaymentGRPC,
		p.Postgres, p.Redis, p.Keycloak, p.FrontendPrimary, p.FrontendSecondary} {
		if paladin[port] {
			t.Errorf("port %d overlaps the Paladin range", port)
		}
	}
}

func itoaT(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
