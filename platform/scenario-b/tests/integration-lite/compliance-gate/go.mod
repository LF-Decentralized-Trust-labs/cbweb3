// Module path is rooted UNDER the compliance module path so that this
// integration-lite suite may import the compliance service's internal/ packages
// (Go's internal rule permits imports from any package rooted at the internal
// parent). This is test-only code; it does not ship with the service.
module github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/integrationlite

go 1.26

require (
	github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance v0.0.0
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto v0.0.0
	github.com/glebarez/sqlite v1.11.0
	google.golang.org/grpc v1.83.1
	google.golang.org/protobuf v1.36.11
	gorm.io/gorm v1.31.0
)

require (
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain v0.0.0 // indirect
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity v0.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/bits-and-blooms/bitset v1.24.6 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/consensys/gnark-crypto v0.21.0 // indirect
	github.com/crate-crypto/go-eth-kzg v1.5.0 // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.8 // indirect
	github.com/ethereum/go-ethereum v1.17.5 // indirect
	github.com/fjl/jsonw v0.1.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/glebarez/go-sqlite v1.21.2 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.9.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/supranational/blst v0.3.16 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	gorm.io/driver/postgres v1.6.0 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
)

replace (
	github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance => ../../../backend/services/compliance
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain => ../../../backend/shared/blockchain
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity => ../../../backend/shared/identity
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto => ../../../backend/shared/proto
)
