package registry

import (
	"fmt"
	"strings"
)

// roleToCode converts a role string to the uint8 code used in the ParticipantRegistry contract.
func roleToCode(role string) uint8 {
	switch role {
	case "ROLE_COMMERCIAL_BANK":
		return 1
	case "ROLE_NOC":
		return 2
	case "ROLE_SUPERVISOR":
		return 3
	case "ROLE_TREASURY":
		return 4
	case "ROLE_GOVERNANCE":
		return 5
	default:
		return 0
	}
}

// encodeRegisterMember ABI-encodes registerMember(address _member, uint8 _role).
// Selector: keccak256("registerMember(address,uint8)")[0:4] = 5c0c4057
func encodeRegisterMember(memberAddr string, roleCode uint8) string {
	const selector = "5c0c4057"
	addrHex := fmt.Sprintf("%064s", strings.TrimPrefix(strings.ToLower(memberAddr), "0x"))
	paddedRole := fmt.Sprintf("%064x", roleCode)
	return "0x" + selector + addrHex + paddedRole
}

// encodeIsAuthorized ABI-encodes isAuthorized(address _member).
// Selector: keccak256("isAuthorized(address)")[0:4] = fe9fbb80
func encodeIsAuthorized(address string) string {
	const selector = "fe9fbb80"
	addrHex := fmt.Sprintf("%064s", strings.TrimPrefix(strings.ToLower(address), "0x"))
	return "0x" + selector + addrHex
}

// encodeGetRole ABI-encodes getRole(address _member).
// Selector: keccak256("getRole(address)")[0:4] = b5dce8cd
func encodeGetRole(address string) string {
	const selector = "b5dce8cd"
	addrHex := fmt.Sprintf("%064s", strings.TrimPrefix(strings.ToLower(address), "0x"))
	return "0x" + selector + addrHex
}
