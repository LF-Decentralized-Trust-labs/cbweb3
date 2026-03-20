PKI_DIR := backend/config/pki
OPENSSL  := openssl

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

pki.check:
	@echo "==> PKI certificate status ($(PKI_DIR)/):"
	@for f in hub-ca.key hub-ca.crt spoke-a-ca.key spoke-a-ca.crt spoke-b-ca.key spoke-b-ca.crt; do \
		if [ -f "$(PKI_DIR)/$$f" ]; then \
			echo "  [OK]     $$f"; \
		else \
			echo "  [MISSING] $$f"; \
		fi; \
	done

pki.clean:
	@echo "==> Removing CA credentials from $(PKI_DIR)/"
	@rm -f $(PKI_DIR)/hub-ca.key $(PKI_DIR)/hub-ca.crt
	@rm -f $(PKI_DIR)/spoke-a-ca.key $(PKI_DIR)/spoke-a-ca.crt
	@rm -f $(PKI_DIR)/spoke-b-ca.key $(PKI_DIR)/spoke-b-ca.crt
	@echo "  Done."

.PHONY: pki.gen-hub pki.gen-spoke-a pki.gen-spoke-b pki.gen-all pki.check pki.clean
