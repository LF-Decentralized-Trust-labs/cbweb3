module github.com/LACNetNetworks/cbweb3-platform/backend/services/auth

go 1.25.5

require (
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain v0.0.0
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity v0.0.0
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto v0.0.0
	github.com/ethereum/go-ethereum v1.17.1
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/redis/go-redis/v9 v9.18.0
	google.golang.org/grpc v1.79.2
)

replace (
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain => ../../shared/blockchain
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity => ../../shared/identity
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto => ../../shared/proto
)

require (
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/bits-and-blooms/bitset v1.20.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/consensys/gnark-crypto v0.18.1 // indirect
	github.com/crate-crypto/go-eth-kzg v1.4.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.6 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/supranational/blst v0.3.16 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
