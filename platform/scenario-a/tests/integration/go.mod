// SPDX-License-Identifier: Apache-2.0
//
// Lightweight, hermetic integration-test module for Scenario A.
//
// The module path is rooted under the payment-orchestrator service prefix on
// purpose: Go's internal-package rule is lexical on the *import path*, so this
// path is what lets the suites import the orchestrator's real
// `internal/grpc/server` constructor and wire the real service in-process.
//
// Everything here is gated behind the `integration_lite` build tag so the
// default `go test` lane never compiles or runs it.
module github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/tests/integration

go 1.26.8

require (
	github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator v0.0.0
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto v0.0.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/ethereum/go-ethereum v1.17.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
)

replace (
	github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator => ../../backend/services/payment-orchestrator
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain => ../../backend/shared/blockchain
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto => ../../backend/shared/proto
)
