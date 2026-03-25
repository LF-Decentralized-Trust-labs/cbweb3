gen-pki: pki.gen-all
gen-pki-central-bank: pki.gen-central-bank
gen-pki-bank-a: pki.gen-bank-a
gen-pki-bank-b: pki.gen-bank-b
check-pki: pki.check
clean-pki: pki.clean
gen-pki-commercial-banks: pki.gen-commercial-banks
check-pki-commercial-banks: pki.check-commercial-banks
clean-pki-commercial-banks: pki.clean-commercial-banks

up-spoke-a: deploy.up-spoke-a
up-besu: deploy.up-besu
up-infra: deploy.up-infra
up: deploy.up

down-spoke-a: deploy.down-spoke-a
down-besu: deploy.down-besu
down-infra: deploy.down-infra
down: deploy.down

up-backend: deploy.up-backend
down-backend: deploy.down-backend
up-backend-bank-a: deploy.up-backend-bank-a
down-backend-bank-a: deploy.down-backend-bank-a
up-backend-bank-b: deploy.up-backend-bank-b
down-backend-bank-b: deploy.down-backend-bank-b
up-backend-central-bank: deploy.up-backend-central-bank
down-backend-central-bank: deploy.down-backend-central-bank
up-backend-entities: deploy.up-backend-entities
down-backend-entities: deploy.down-backend-entities
validate-backend-entities: deploy.validate-backend-entities

.PHONY: gen-pki gen-pki-central-bank gen-pki-bank-a gen-pki-bank-b check-pki clean-pki gen-pki-commercial-banks check-pki-commercial-banks clean-pki-commercial-banks up-spoke-a up-besu up-infra up down-spoke-a down-besu down-infra down up-backend down-backend up-backend-bank-a down-backend-bank-a up-backend-bank-b down-backend-bank-b up-backend-central-bank down-backend-central-bank up-backend-entities down-backend-entities validate-backend-entities
