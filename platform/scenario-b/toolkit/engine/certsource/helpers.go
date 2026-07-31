package certsource

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
)

// roleCommercialBank is the OU a leaf CSR must carry to be issued.
const roleCommercialBank = "ROLE_COMMERCIAL_BANK"

// parseAndVerifyCSR decodes a PEM PKCS#10 CSR and verifies its self-signature.
func parseAndVerifyCSR(csrPEM []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, ErrInvalidCSR
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, ErrInvalidCSR
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, ErrInvalidCSR
	}
	return csr, nil
}

func hasRole(ous []string, role string) bool {
	for _, ou := range ous {
		if ou == role {
			return true
		}
	}
	return false
}

// randSerial returns a random 128-bit certificate serial number.
func randSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}
