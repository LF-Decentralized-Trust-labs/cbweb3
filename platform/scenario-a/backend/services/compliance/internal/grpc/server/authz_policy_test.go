// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"regexp"
	"strings"
	"testing"

	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
)

// mutatingRPC matches the verbs this service uses for state-changing operations
// (admitting or freezing a participant, halting the system, on-chain registration). It is deliberately broad: the test below uses it to fail
// when a NEW such RPC is added without being classified, which is the failure mode a
// hand-maintained list has (the list stops matching the service and nobody notices).
var mutatingRPC = regexp.MustCompile(`^(Approve|Manage|Toggle|Update|Register|Issue|Sign|Upsert|Create|Delete)`)

// Every mutating RPC must carry a per-method restriction. Adding one without listing
// it in serverPolicy leaves it open to any authenticated peer — exactly the gap R2-H-8
// closes — and this test is what makes that a build failure rather than a discovery.
func TestServerPolicy_CoversEveryMutatingRPC(t *testing.T) {
	policy, ok := serverPolicy().(authz.MethodPolicy)
	if !ok {
		t.Fatalf("serverPolicy must be a MethodPolicy, got %T", serverPolicy())
	}

	var unrestricted []string
	for _, m := range compliancv1.ComplianceService_ServiceDesc.Methods {
		if !mutatingRPC.MatchString(m.MethodName) {
			continue
		}
		full := "/" + compliancv1.ComplianceService_ServiceDesc.ServiceName + "/" + m.MethodName
		if _, restricted := policy.ByMethod[full]; !restricted {
			unrestricted = append(unrestricted, m.MethodName)
		}
	}
	if len(unrestricted) > 0 {
		t.Fatalf("these mutating RPCs have no per-method restriction: %s\n"+
			"Add them to serverPolicy, or rename them if they do not move value.",
			strings.Join(unrestricted, ", "))
	}
}

// The restriction has to bite: an authenticated peer that is not the gateway must be
// refused on a value-moving method, and the gateway must still be admitted.
func TestServerPolicy_RefusesANonGatewayCallerOnApproveKYC(t *testing.T) {
	policy := serverPolicy()
	mint := compliancv1.ComplianceService_ApproveKYC_FullMethodName

	if err := policy.Authorize(context.Background(), &authz.Identity{Subject: callerGateway, Method: "mtls"}, mint); err != nil {
		t.Fatalf("the gateway must be admitted on %s: %v", mint, err)
	}
	for _, subject := range []string{"payment-orchestrator", "attacker"} {
		id := &authz.Identity{Subject: subject, Method: "mtls"}
		if err := policy.Authorize(context.Background(), id, mint); err == nil {
			t.Fatalf("caller %q must not be authorized to call %s", subject, mint)
		}
	}
}

// Reads keep the baseline policy: tightening them would break the read paths other
// services legitimately use, and they confer no authority.
func TestServerPolicy_LeavesReadsOnTheBaseline(t *testing.T) {
	policy := serverPolicy()
	id := &authz.Identity{Subject: "payment-orchestrator", Method: "mtls"}
	if err := policy.Authorize(context.Background(), id, compliancv1.ComplianceService_ListParticipants_FullMethodName); err != nil {
		t.Fatalf("an authenticated caller must still be able to read: %v", err)
	}
}
