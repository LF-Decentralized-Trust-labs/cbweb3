.PHONY: spk01.up spk01.found spk01.bundle spk01.join spk01.verify spk01.down spk01.clean

# Work from the spike root directory
SPIKE_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

spk01.up: spk01.found spk01.bundle spk01.join

spk01.found:
	bash scripts/start-found.sh

spk01.bundle:
	bash scripts/extract-bundle.sh

spk01.join:
	bash scripts/start-join.sh

spk01.verify:
	@echo "=== T1: genesis idempotency ==="
	bash scripts/verify-genesis-idempotency.sh ./data/genesis.json
	@echo ""
	@echo "=== T2: block production ==="
	bash scripts/verify-peering.sh found
	@echo ""
	@echo "=== T3: enode audit ==="
	bash scripts/verify-enode.sh http://localhost:8645
	@echo ""
	@echo "=== T4: cross-stack peering ==="
	bash scripts/verify-peering.sh join
	bash scripts/verify-network-isolation.sh
	@echo ""
	@echo "=== T5: cacti RPC isolation ==="
	docker compose -f compose/cacti-isolated.yml run --rm cacti-smoke bash /scripts/verify-cacti-rpc.sh
	@echo ""
	@echo "=== T6: bundle clean ==="
	bash scripts/verify-bundle.sh bundles/spoke-spk01.bundle.yaml
	@echo ""
	@echo "ALL TESTS PASSED"

spk01.down:
	docker compose -f compose/cacti-isolated.yml down -v 2>/dev/null || true
	docker compose -f compose/stack-join.yml down -v
	docker compose -f compose/stack-found.yml down -v

spk01.clean: spk01.down
	rm -rf data/ bundles/*.yaml
	rm -f bundles/*.yaml
