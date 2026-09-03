// Module path is rooted UNDER the payment-orchestrator module path so that this
// integration-lite suite may import the orchestrator's internal/ packages (Go's
// internal rule permits imports from any package rooted at the internal parent).
// This is test-only code; it does not ship with the service.
module github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/integrationlite

go 1.26

require (
	github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator v0.0.0
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto v0.0.0
	github.com/glebarez/sqlite v1.11.0
	google.golang.org/grpc v1.83.1
	gorm.io/gorm v1.31.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ethereum/go-ethereum v1.17.5 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)

replace (
	github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator => ../../../backend/services/payment-orchestrator
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain => ../../../backend/shared/blockchain
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto => ../../../backend/shared/proto
)
