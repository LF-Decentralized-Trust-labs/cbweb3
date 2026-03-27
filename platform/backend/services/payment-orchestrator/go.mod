module github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator

go 1.25.5

replace (
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain => ../../shared/blockchain
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto => ../../shared/proto
)

require (
	github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.79.3
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
