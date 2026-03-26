# Short aliases for common operations

up: deploy.up
down: deploy.down
up-infra: deploy.up-infra
down-infra: deploy.down-infra
up-besu: deploy.up-besu
down-besu: deploy.down-besu
up-backend: deploy.up-backend
down-backend: deploy.down-backend

gen-pki: pki.gen-all
check-pki: pki.check
clean-pki: pki.clean

.PHONY: up down up-infra down-infra up-besu down-besu up-backend down-backend \
	gen-pki check-pki clean-pki
