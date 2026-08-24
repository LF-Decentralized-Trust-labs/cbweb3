# SPDX-License-Identifier: Apache-2.0
#
# Local CI runner (act). This file used to hold the legacy `deploy/local` bring-up —
# per-layer and per-entity targets that duplicated what the toolkit does. That path was
# removed: the toolkit (`samples/deploy-all.sh`, `cbweb3b apply`) is
# the only way to bring a stack up now.
#
# Why the duplication had to go. Every hardening landed twice or not at all — the Besu
# pin reached the toolkit and not these scripts (15 places still on `:latest`), and
# per-entity credentials and Redis auth only reached them because one PR touched both
# trees deliberately. One path means one place to harden.
#
# See docs/decisions/ADR-006 (hub network isolation) for the topology the toolkit
# provisions, and the deployment runbook for the procedure.

ARGS ?=

## deploy.build-ci-runner: build the local GitHub Actions runner image (act)
deploy.build-ci-runner:
	@docker build -t cbweb3-act-runner:latest -f ./tools/act/Dockerfile .

## deploy.ci-local: run a workflow locally with act (ARGS passes act arguments)
deploy.ci-local: deploy.build-ci-runner
	@docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(shell pwd):/src \
		cbweb3-act-runner:latest \
		-P ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-22.04 \
		$(ARGS)
