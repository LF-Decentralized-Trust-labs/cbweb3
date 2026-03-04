// This file verifies Ethereum wallet signatures for wallet binding requests.
package auth

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// VerifyWalletSignature validates that the signature matches subject and wallet.
func VerifyWalletSignature(subject, walletAddress, signature string) error {
	if !common.IsHexAddress(walletAddress) {
		return fmt.Errorf("wallet address is invalid")
	}

	sigHex := strings.TrimPrefix(signature, "0x")
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return err
	}
	if len(sig) != 65 {
		return fmt.Errorf("signature must have 65 bytes")
	}
	if sig[64] == 27 || sig[64] == 28 {
		sig[64] -= 27
	}
	if sig[64] != 0 && sig[64] != 1 {
		return fmt.Errorf("invalid recovery id")
	}

	message := walletBindMessage(subject, walletAddress)
	hash := accounts.TextHash([]byte(message))
	pubKey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return err
	}
	recovered := crypto.PubkeyToAddress(*pubKey).Hex()
	expected := common.HexToAddress(walletAddress).Hex()
	if !strings.EqualFold(recovered, expected) {
		return fmt.Errorf("signature does not match wallet")
	}

	return nil
}

// walletBindMessage returns the canonical wallet binding message to sign.
func walletBindMessage(subject, walletAddress string) string {
	return fmt.Sprintf("CBWEB3_WALLET_BIND:%s:%s", subject, strings.ToLower(walletAddress))
}

