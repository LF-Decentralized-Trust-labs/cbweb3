# Short aliases for common operations

up: deploy.up
down: deploy.down
up-infra: deploy.up-infra
down-infra: deploy.down-infra
up-besu: deploy.up-besu
down-besu: deploy.down-besu
up-backend: deploy.up-backend
down-backend: deploy.down-backend

.PHONY: gen-pki gen-pki-hub gen-pki-spoke-a gen-pki-spoke-b check-pki clean-pki gen-pki-commercial-banks check-pki-commercial-banks clean-pki-commercial-banks up-hub up-spoke-a up-spoke-b up-besu up-infra up up-minimal down-hub down-spoke-a down-spoke-b down-besu down-infra down down-minimal up-backend down-backend up-backend-spoke-a down-backend-spoke-a up-backend-spoke-b down-backend-spoke-b up-backend-hub down-backend-hub up-backend-domains down-backend-domains validate-backend-domains test-api-gateway test-auth test-compliance test-identity test-data-access test-services
