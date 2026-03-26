# Short aliases for common operations

up: deploy.up
down: deploy.down
up-infra: deploy.up-infra
down-infra: deploy.down-infra
up-besu: deploy.up-besu
down-besu: deploy.down-besu
up-backend: deploy.up-backend
down-backend: deploy.down-backend

paladin-start-spoke-a: paladin.start-spoke-a
paladin-stop-spoke-a: paladin.stop-spoke-a
paladin-start-spoke-b: paladin.start-spoke-b
paladin-stop-spoke-b: paladin.stop-spoke-b
paladin-deploy-contracts-spoke-a: paladin.deploy-contracts-spoke-a
paladin-deploy-contracts-spoke-b: paladin.deploy-contracts-spoke-b
paladin-register-spoke-a: paladin.register-nodes-spoke-a
paladin-register-spoke-b: paladin.register-nodes-spoke-b

.PHONY: gen-pki gen-pki-hub gen-pki-spoke-a gen-pki-spoke-b check-pki clean-pki gen-pki-commercial-banks check-pki-commercial-banks clean-pki-commercial-banks up-hub up-spoke-a up-spoke-b up-besu up-infra up up-minimal down-hub down-spoke-a down-spoke-b down-besu down-infra down down-minimal up-backend down-backend up-backend-spoke-a down-backend-spoke-a up-backend-spoke-b down-backend-spoke-b up-backend-hub down-backend-hub up-backend-domains down-backend-domains validate-backend-domains test-api-gateway test-auth test-compliance test-identity test-data-access test-services paladin-start-spoke-a paladin-stop-spoke-a paladin-start-spoke-b paladin-stop-spoke-b paladin-deploy-contracts-spoke-a paladin-deploy-contracts-spoke-b paladin-register-spoke-a paladin-register-spoke-b
