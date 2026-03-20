gen-pki: pki.gen-all
gen-pki-hub: pki.gen-hub
gen-pki-spoke-a: pki.gen-spoke-a
gen-pki-spoke-b: pki.gen-spoke-b
check-pki: pki.check
clean-pki: pki.clean
gen-pki-commercial-banks: pki.gen-commercial-banks
check-pki-commercial-banks: pki.check-commercial-banks
clean-pki-commercial-banks: pki.clean-commercial-banks

up-hub: deploy.up-hub
up-spoke-a: deploy.up-spoke-a
up-spoke-b: deploy.up-spoke-b
up-besu: deploy.up-besu
up-infra: deploy.up-infra
up: deploy.up
up-minimal: deploy.up-minimal

down-hub: deploy.down-hub
down-spoke-a: deploy.down-spoke-a
down-spoke-b: deploy.down-spoke-b
down-besu: deploy.down-besu
down-infra: deploy.down-infra
down: deploy.down
down-minimal: deploy.down-minimal

up-backend: deploy.up-backend
down-backend: deploy.down-backend
up-backend-spoke-a: deploy.up-backend-spoke-a
down-backend-spoke-a: deploy.down-backend-spoke-a
up-backend-spoke-b: deploy.up-backend-spoke-b
down-backend-spoke-b: deploy.down-backend-spoke-b
up-backend-hub: deploy.up-backend-hub
down-backend-hub: deploy.down-backend-hub
up-backend-domains: deploy.up-backend-domains
down-backend-domains: deploy.down-backend-domains
validate-backend-domains: deploy.validate-backend-domains
test-api-gateway: deploy.test-api-gateway
test-auth: deploy.test-auth
test-compliance: deploy.test-compliance
test-identity: deploy.test-identity
test-data-access: deploy.test-data-access
test-services: deploy.test-services

.PHONY: gen-pki gen-pki-hub gen-pki-spoke-a gen-pki-spoke-b check-pki clean-pki gen-pki-commercial-banks check-pki-commercial-banks clean-pki-commercial-banks up-hub up-spoke-a up-spoke-b up-besu up-infra up up-minimal down-hub down-spoke-a down-spoke-b down-besu down-infra down down-minimal up-backend down-backend up-backend-spoke-a down-backend-spoke-a up-backend-spoke-b down-backend-spoke-b up-backend-hub down-backend-hub up-backend-domains down-backend-domains validate-backend-domains test-api-gateway test-auth test-compliance test-identity test-data-access test-services
