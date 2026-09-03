.PHONY: spk02.up spk02.found spk02.bundle spk02.join spk02.baseline spk02.vote spk02.paladin-join spk02.verify spk02.rolling-restart spk02.down spk02.clean

SPIKE_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

# Full POC: found stack → bundle → join (non-validator) → baseline capture →
# QBFT vote (promote to validator) → Paladin live join. After this completes,
# `make spk02.verify` exercises T1–T6.
spk02.up: spk02.found spk02.bundle spk02.join spk02.baseline spk02.vote spk02.paladin-join

spk02.found:
	bash scripts/start-found.sh

spk02.bundle:
	bash scripts/extract-bundle.sh

spk02.join:
	bash scripts/join-besu.sh

# Capture container start times for all existing found-stack containers AFTER the
# joiner is up but BEFORE any vote or Paladin join — this is the baseline that T4
# and T5 compare against to prove no existing container restarted.
spk02.baseline:
	bash scripts/record-container-start-times.sh

spk02.vote:
	bash scripts/vote-in-validator.sh

spk02.paladin-join:
	bash scripts/join-paladin.sh

spk02.verify:
	@echo "=== T1: QBFT validator set ==="
	bash scripts/verify-qbft.sh
	@echo ""
	@echo "=== T2: block continuity ==="
	bash scripts/verify-qbft.sh --t2
	@echo ""
	@echo "=== T3: Paladin mTLS cross-node ==="
	bash scripts/verify-paladin-join.sh --t3
	@echo ""
	@echo "=== T4: no existing Paladin restart ==="
	bash scripts/verify-paladin-join.sh --t4
	@echo ""
	@echo "=== T5: no Besu validator restart ==="
	bash scripts/verify-restart-scope.sh
	@echo ""
	@echo "=== T6: ADR evidence table complete ==="
	bash scripts/verify-adr-evidence.sh
	@echo ""
	@echo "ALL TESTS PASSED"

spk02.rolling-restart:
	bash scripts/rolling-restart.sh

spk02.down:
	docker compose -f compose/stack-join.yml down -v 2>/dev/null || true
	docker compose -f compose/stack-found.yml down -v 2>/dev/null || true

spk02.clean: spk02.down
	rm -f data/genesis.json data/key* data/.deployed-addrs.env data/baseline-start-times.json bundles/*.yaml
	rm -rf data/paladin/*/config.yaml data/paladin/*/tls.key data/paladin/*/tls.crt
