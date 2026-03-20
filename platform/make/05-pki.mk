PKI_DIR := backend/config/pki
OPENSSL  := openssl
COMMERCIAL_BANK_IDS := bank-001 bank-002 bank-003 bank-004 bank-005 bank-006
COMMERCIAL_BANK_GEN_TARGETS := $(addprefix pki.gen-commercial-bank-,$(COMMERCIAL_BANK_IDS))

# Guard: skip generation if .key already exists, unless FORCE=1 is set
define gen_ca
	@if [ -f "$(1)" ] && [ "$(FORCE)" != "1" ]; then \
		echo "  [skip] $(1) already exists (run with FORCE=1 to regenerate)"; \
	else \
		echo "  Generating $(2) private key..."; \
		$(OPENSSL) ecparam -genkey -name prime256v1 -noout -out "$(1)"; \
		echo "  Generating $(2) self-signed certificate..."; \
		$(OPENSSL) req -new -x509 \
			-key "$(1)" \
			-out "$(3)" \
			-days 3650 \
			-subj "$(4)"; \
		echo "  Done: $(3)"; \
	fi
endef

# Guard: skip generation if participant certificate already exists, unless FORCE=1
define gen_commercial_bank_cert
	@bank="$(1)"; \
	ca_key="$(PKI_DIR)/$$bank-ca.key"; \
	ca_crt="$(PKI_DIR)/$$bank-ca.crt"; \
	participant_key="$(PKI_DIR)/$$bank.key"; \
	participant_csr="$(PKI_DIR)/$$bank.csr"; \
	participant_crt="$(PKI_DIR)/$$bank.crt"; \
	serial_hex="$$(openssl rand -hex 8)"; \
	if [ -f "$$participant_crt" ] && [ "$(FORCE)" != "1" ]; then \
		echo "  [skip] $$participant_crt already exists (run with FORCE=1 to regenerate)"; \
	else \
		echo "  Generating $$bank CA private key..."; \
		$(OPENSSL) ecparam -genkey -name prime256v1 -noout -out "$$ca_key"; \
		echo "  Generating $$bank CA certificate..."; \
		$(OPENSSL) req -new -x509 \
			-key "$$ca_key" \
			-out "$$ca_crt" \
			-days 3650 \
			-subj "/CN=$$bank-CA/O=$$bank/C=BR"; \
		echo "  Generating $$bank participant private key..."; \
		$(OPENSSL) ecparam -genkey -name prime256v1 -noout -out "$$participant_key"; \
		echo "  Generating $$bank participant CSR..."; \
		$(OPENSSL) req -new \
			-key "$$participant_key" \
			-out "$$participant_csr" \
			-subj "/CN=$$bank/O=$$bank/OU=ROLE_COMMERCIAL_BANK/C=BR"; \
		echo "  Signing $$bank participant certificate with $$bank CA..."; \
		$(OPENSSL) x509 -req \
			-in "$$participant_csr" \
			-CA "$$ca_crt" \
			-CAkey "$$ca_key" \
			-set_serial "0x$$serial_hex" \
			-out "$$participant_crt" \
			-days 1825 \
			-sha256; \
		echo "  Done: $$participant_crt"; \
	fi
endef

pki.gen-hub:
	@echo "==> Hub CA"
	$(call gen_ca,$(PKI_DIR)/hub-ca.key,Hub,$(PKI_DIR)/hub-ca.crt,/CN=CBWeb3-Hub-CA/O=CBWeb3-Hub/C=BR)

pki.gen-spoke-a:
	@echo "==> Spoke A (BCB-A) CA"
	$(call gen_ca,$(PKI_DIR)/spoke-a-ca.key,SpokeA,$(PKI_DIR)/spoke-a-ca.crt,/CN=CBWeb3-SpokeA-CA/O=BCB-A/C=BR)

pki.gen-spoke-b:
	@echo "==> Spoke B (BCB-B) CA"
	$(call gen_ca,$(PKI_DIR)/spoke-b-ca.key,SpokeB,$(PKI_DIR)/spoke-b-ca.crt,/CN=CBWeb3-SpokeB-CA/O=BCB-B/C=BR)

pki.gen-all: pki.gen-hub pki.gen-spoke-a pki.gen-spoke-b
	@echo "==> All CA credentials generated in $(PKI_DIR)/"

pki.gen-commercial-bank-%:
	@echo "==> Commercial Bank PKI: $*"
	$(call gen_commercial_bank_cert,$*)

pki.gen-commercial-banks: $(COMMERCIAL_BANK_GEN_TARGETS)
	@echo "==> Commercial bank certificates generated in $(PKI_DIR)/"

pki.check:
	@echo "==> PKI certificate status ($(PKI_DIR)/):"
	@for f in hub-ca.key hub-ca.crt spoke-a-ca.key spoke-a-ca.crt spoke-b-ca.key spoke-b-ca.crt; do \
		if [ -f "$(PKI_DIR)/$$f" ]; then \
			echo "  [OK]     $$f"; \
		else \
			echo "  [MISSING] $$f"; \
		fi; \
	done

pki.check-commercial-banks:
	@echo "==> Commercial bank PKI status ($(PKI_DIR)/):"
	@for bank in $(COMMERCIAL_BANK_IDS); do \
		for suffix in -ca.key -ca.crt .key .csr .crt; do \
			f="$$bank$$suffix"; \
			if [ -f "$(PKI_DIR)/$$f" ]; then \
				echo "  [OK]      $$f"; \
			else \
				echo "  [MISSING] $$f"; \
			fi; \
		done; \
	done

pki.clean:
	@echo "==> Removing CA credentials from $(PKI_DIR)/"
	@rm -f $(PKI_DIR)/hub-ca.key $(PKI_DIR)/hub-ca.crt
	@rm -f $(PKI_DIR)/spoke-a-ca.key $(PKI_DIR)/spoke-a-ca.crt
	@rm -f $(PKI_DIR)/spoke-b-ca.key $(PKI_DIR)/spoke-b-ca.crt
	@echo "  Done."

pki.clean-commercial-banks:
	@echo "==> Removing commercial bank credentials from $(PKI_DIR)/"
	@rm -f $(foreach bank,$(COMMERCIAL_BANK_IDS),$(PKI_DIR)/$(bank)-ca.key $(PKI_DIR)/$(bank)-ca.crt $(PKI_DIR)/$(bank)-ca.srl $(PKI_DIR)/$(bank).key $(PKI_DIR)/$(bank).csr $(PKI_DIR)/$(bank).crt)
	@echo "  Done."

.PHONY: pki.gen-hub pki.gen-spoke-a pki.gen-spoke-b pki.gen-all pki.check pki.clean pki.gen-commercial-banks pki.check-commercial-banks pki.clean-commercial-banks
