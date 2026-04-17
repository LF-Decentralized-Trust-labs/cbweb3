package paladin

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.PenteClientPort = (*PenteClient)(nil)

// PenteClient is a lightweight HTTP adapter for bilateral FX private contexts.
type PenteClient struct {
	baseURL                  string
	httpClient               *http.Client
	defaultFXAgreementTarget ports.PenteFXTarget
	identity                 string
}

// PenteClientConfig configures the remote Pente gateway URL.
type PenteClientConfig struct {
	BaseURL            string
	FXAgreementAddress string
	Identity           string
}

func NewPenteClient(cfg PenteClientConfig) *PenteClient {
	return &PenteClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		defaultFXAgreementTarget: ports.PenteFXTarget{
			ContractAddress: normalizeAddress(cfg.FXAgreementAddress),
		},
		identity: cfg.Identity,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func normalizeAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, "0x") {
		return strings.ToLower(addr)
	}
	return "0x" + strings.ToLower(addr)
}

func bytes32ToHex(v [32]byte) string {
	return "0x" + hex.EncodeToString(v[:])
}

func ratToDecimalString(v *big.Rat) string {
	if v == nil {
		return ""
	}
	if v.IsInt() {
		return v.Num().String()
	}
	return v.FloatString(18)
}

func extractTxHash(raw json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"tx_hash", "txHash", "transaction_hash", "transactionHash", "hash", "id"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func (c *PenteClient) resolveFXTarget(ctx context.Context) (ports.PenteFXTarget, error) {
	if target, ok := ports.PenteFXTargetFromContext(ctx); ok {
		target.ContractAddress = normalizeAddress(target.ContractAddress)
		if target.ContractAddress != "" {
			return target, nil
		}
	}
	if c.defaultFXAgreementTarget.ContractAddress != "" {
		return c.defaultFXAgreementTarget, nil
	}
	return ports.PenteFXTarget{}, fmt.Errorf("missing Pente FX target in context")
}

func (c *PenteClient) callFXAgreement(ctx context.Context, method, path string, payload any, target ports.PenteFXTarget) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal Pente FX payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create Pente FX request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if target.GroupID != "" {
		httpReq.Header.Set("X-Pente-Group-Id", target.GroupID)
	}
	if c.identity != "" {
		httpReq.Header.Set("X-Paladin-Identity", c.identity)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call Pente FX endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Pente FX endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	txHash := extractTxHash(respBody)
	return txHash, nil
}

func (c *PenteClient) EnsureFXContext(ctx context.Context, req ports.PenteContextRequest) (*ports.PenteContextResult, error) {
	payload := map[string]string{
		"trade_id":     req.TradeID,
		"originator":   req.Originator,
		"counterparty": req.Counterparty,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Pente request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/pente/fx/context", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Pente request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call Pente context endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Pente context endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		GroupID         string `json:"group_id"`
		ContractAddress string `json:"contract_address"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("unmarshal Pente response: %w", err)
	}
	if out.GroupID == "" || out.ContractAddress == "" {
		return nil, fmt.Errorf("invalid Pente response: group_id/contract_address required")
	}
	return &ports.PenteContextResult{
		GroupID:         out.GroupID,
		ContractAddress: out.ContractAddress,
	}, nil
}

func (c *PenteClient) Propose(ctx context.Context, params ports.FXProposalParams) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"trade_id":         bytes32ToHex(params.TradeID),
		"originator":       params.Originator.Hex(),
		"counterparty_b":   params.CounterpartyB.Hex(),
		"settlement_agent": params.SettlementAgent.Hex(),
		"custodian":        params.Custodian.Hex(),
		"beneficiary":      params.Beneficiary.Hex(),
		"origin_amount":    ratToDecimalString(new(big.Rat).SetInt(params.OriginAmount)),
		"counter_amount":   ratToDecimalString(new(big.Rat).SetInt(params.CounterAmount)),
		"origin_currency":  strings.TrimRight(string(params.OriginCurrency[:]), "\x00"),
		"counter_currency": strings.TrimRight(string(params.CounterCurrency[:]), "\x00"),
		"rate":             ratToDecimalString(new(big.Rat).SetInt(params.Rate)),
		"expiry_date":      params.ExpiryDate.String(),
	}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/propose", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) ProposeOnBehalf(ctx context.Context, params ports.FXProposalParams) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"trade_id":         bytes32ToHex(params.TradeID),
		"originator":       params.Originator.Hex(),
		"counterparty_b":   params.CounterpartyB.Hex(),
		"settlement_agent": params.SettlementAgent.Hex(),
		"custodian":        params.Custodian.Hex(),
		"beneficiary":      params.Beneficiary.Hex(),
		"origin_amount":    ratToDecimalString(new(big.Rat).SetInt(params.OriginAmount)),
		"counter_amount":   ratToDecimalString(new(big.Rat).SetInt(params.CounterAmount)),
		"origin_currency":  strings.TrimRight(string(params.OriginCurrency[:]), "\x00"),
		"counter_currency": strings.TrimRight(string(params.CounterCurrency[:]), "\x00"),
		"rate":             ratToDecimalString(new(big.Rat).SetInt(params.Rate)),
		"expiry_date":      params.ExpiryDate.String(),
		"on_behalf":        true,
	}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/propose", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) Accept(ctx context.Context, tradeID [32]byte) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]string{"trade_id": bytes32ToHex(tradeID)}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/accept", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) AcceptOnBehalf(ctx context.Context, tradeID [32]byte) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{"trade_id": bytes32ToHex(tradeID), "on_behalf": true}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/accept", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) Reject(ctx context.Context, tradeID [32]byte) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]string{"trade_id": bytes32ToHex(tradeID)}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/reject", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) RejectOnBehalf(ctx context.Context, tradeID [32]byte) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{"trade_id": bytes32ToHex(tradeID), "on_behalf": true}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/reject", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) Cancel(ctx context.Context, tradeID [32]byte) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]string{"trade_id": bytes32ToHex(tradeID)}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/cancel", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) Settle(ctx context.Context, tradeID [32]byte) (string, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]string{"trade_id": bytes32ToHex(tradeID)}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/settle", target.ContractAddress)
	return c.callFXAgreement(ctx, http.MethodPost, path, payload, target)
}

func (c *PenteClient) GetFXAgreement(ctx context.Context, tradeID string) (*ports.PenteFXAgreementState, error) {
	target, err := c.resolveFXTarget(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/pente/fx/agreements/%s/%s", target.ContractAddress, tradeID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create Pente FX get request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if target.GroupID != "" {
		httpReq.Header.Set("X-Pente-Group-Id", target.GroupID)
	}
	if c.identity != "" {
		httpReq.Header.Set("X-Paladin-Identity", c.identity)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call Pente FX get endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Pente FX get endpoint returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out ports.PenteFXAgreementState
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("unmarshal Pente FX state: %w", err)
	}
	return &out, nil
}
