// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// ---------------------------------------------------------------------------
// CurrencyService
// ---------------------------------------------------------------------------

type fakeCurrencyClient struct {
	all         []domain.CurrencyEntry
	allErr      error
	registerTx  string
	registerErr error
	removeTx    string
	removeErr   error
}

func (f *fakeCurrencyClient) RegisterCurrency(_ context.Context, _, _, _, _ string) (string, error) {
	return f.registerTx, f.registerErr
}
func (f *fakeCurrencyClient) RemoveCurrency(_ context.Context, _ string) (string, error) {
	return f.removeTx, f.removeErr
}
func (f *fakeCurrencyClient) GetAllCurrencies(_ context.Context) ([]domain.CurrencyEntry, error) {
	return f.all, f.allErr
}

func TestCurrencyService_Register_Success(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{registerTx: "0xtx"})
	res, err := svc.RegisterCurrency(context.Background(), CurrencyRegisterRequest{
		Symbol: "BRL", CountryName: "Brazil", TokenAddress: "0xabc", ProposerCB: "cb-br",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TxHash != "0xtx" || res.Symbol != "BRL" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestCurrencyService_Register_NilClient(t *testing.T) {
	svc := NewCurrencyService(nil)
	if _, err := svc.RegisterCurrency(context.Background(), CurrencyRegisterRequest{Symbol: "BRL", CountryName: "B", TokenAddress: "0x", ProposerCB: "cb"}); err == nil {
		t.Fatal("expected nil-client error")
	}
}

func TestCurrencyService_Register_Validation(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{})
	if _, err := svc.RegisterCurrency(context.Background(), CurrencyRegisterRequest{Symbol: ""}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCurrencyService_Register_DuplicateSymbol(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{all: []domain.CurrencyEntry{{Symbol: "BRL", TokenAddress: "0xother"}}})
	_, err := svc.RegisterCurrency(context.Background(), CurrencyRegisterRequest{Symbol: "brl", CountryName: "B", TokenAddress: "0xnew", ProposerCB: "cb"})
	if !errors.Is(err, ErrCurrencyAlreadyExists) {
		t.Fatalf("expected ErrCurrencyAlreadyExists, got %v", err)
	}
}

func TestCurrencyService_Register_DuplicateToken(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{all: []domain.CurrencyEntry{{Symbol: "USD", TokenAddress: "0xABC"}}})
	_, err := svc.RegisterCurrency(context.Background(), CurrencyRegisterRequest{Symbol: "BRL", CountryName: "B", TokenAddress: "0xabc", ProposerCB: "cb"})
	if !errors.Is(err, ErrTokenAlreadyRegistered) {
		t.Fatalf("expected ErrTokenAlreadyRegistered, got %v", err)
	}
}

func TestCurrencyService_Register_OnChainErrorMapped(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{registerErr: errors.New("execution reverted: AlreadyExists")})
	_, err := svc.RegisterCurrency(context.Background(), CurrencyRegisterRequest{Symbol: "BRL", CountryName: "B", TokenAddress: "0x", ProposerCB: "cb"})
	if !errors.Is(err, ErrCurrencyAlreadyExists) {
		t.Fatalf("expected mapped ErrCurrencyAlreadyExists, got %v", err)
	}
}

func TestCurrencyService_Remove(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{removeTx: "0xrm"})
	res, err := svc.RemoveCurrency(context.Background(), CurrencyRemoveRequest{Symbol: "BRL"})
	if err != nil || res.TxHash != "0xrm" {
		t.Fatalf("unexpected: res=%+v err=%v", res, err)
	}

	if _, err := svc.RemoveCurrency(context.Background(), CurrencyRemoveRequest{Symbol: ""}); err == nil {
		t.Fatal("expected validation error")
	}

	svcNil := NewCurrencyService(nil)
	if _, err := svcNil.RemoveCurrency(context.Background(), CurrencyRemoveRequest{Symbol: "BRL"}); err == nil {
		t.Fatal("expected nil-client error")
	}

	svcErr := NewCurrencyService(&fakeCurrencyClient{removeErr: errors.New("NotFound")})
	if _, err := svcErr.RemoveCurrency(context.Background(), CurrencyRemoveRequest{Symbol: "BRL"}); !errors.Is(err, ErrCurrencyNotFound) {
		t.Fatalf("expected ErrCurrencyNotFound, got %v", err)
	}
}

func TestCurrencyService_List(t *testing.T) {
	svc := NewCurrencyService(&fakeCurrencyClient{all: []domain.CurrencyEntry{{Symbol: "BRL"}}})
	out, err := svc.ListCurrencies(context.Background())
	if err != nil || len(out) != 1 {
		t.Fatalf("unexpected: out=%v err=%v", out, err)
	}

	svcNilEntries := NewCurrencyService(&fakeCurrencyClient{all: nil})
	out, err = svcNilEntries.ListCurrencies(context.Background())
	if err != nil || out == nil || len(out) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v err %v", out, err)
	}

	svcErr := NewCurrencyService(&fakeCurrencyClient{allErr: errors.New("rpc")})
	if _, err := svcErr.ListCurrencies(context.Background()); err == nil {
		t.Fatal("expected list error")
	}

	if _, err := NewCurrencyService(nil).ListCurrencies(context.Background()); err == nil {
		t.Fatal("expected nil-client error")
	}
}

func TestMapCurrencyOnChainError(t *testing.T) {
	cases := map[string]error{
		"AlreadyExists":          ErrCurrencyAlreadyExists,
		"TokenAlreadyRegistered": ErrTokenAlreadyRegistered,
		"NotFound":               ErrCurrencyNotFound,
		"Unauthorized":           ErrCurrencyUnauthorized,
	}
	for msg, want := range cases {
		if got := mapCurrencyOnChainError(errors.New(msg), "S", "0x"); !errors.Is(got, want) {
			t.Fatalf("mapCurrencyOnChainError(%q) = %v, want %v", msg, got, want)
		}
	}
	if got := mapCurrencyOnChainError(errors.New("some other revert"), "S", "0x"); errors.Is(got, ErrCurrencyNotFound) {
		t.Fatal("default branch should not map to a sentinel")
	}
}

// ---------------------------------------------------------------------------
// PairService
// ---------------------------------------------------------------------------

type fakePairClient struct {
	proposeTx  string
	proposeErr error
	confirmTx  string
	confirmErr error
	active     []domain.PairEntry
	activeErr  error
}

func (f *fakePairClient) ProposePair(_ context.Context, _, _, _, _ string) (string, error) {
	return f.proposeTx, f.proposeErr
}
func (f *fakePairClient) ConfirmPair(_ context.Context, _ string) (string, error) {
	return f.confirmTx, f.confirmErr
}
func (f *fakePairClient) GetAllActivePairs(_ context.Context) ([]domain.PairEntry, error) {
	return f.active, f.activeErr
}
func (f *fakePairClient) GetAllPairs(_ context.Context) ([]domain.PairEntry, error) {
	return f.active, f.activeErr
}

type fakePairRepo struct {
	byID        *domain.PairProposal
	byTokenPair *domain.PairProposal
	all         []domain.PairProposal
	activateErr error
	created     []*domain.PairProposal
}

func (f *fakePairRepo) Create(_ context.Context, p *domain.PairProposal) error {
	f.created = append(f.created, p)
	return nil
}
func (f *fakePairRepo) FindByPairID(_ context.Context, _ string) (*domain.PairProposal, error) {
	return f.byID, nil
}
func (f *fakePairRepo) FindByTokenPair(_ context.Context, _, _ string) (*domain.PairProposal, error) {
	return f.byTokenPair, nil
}
func (f *fakePairRepo) Activate(_ context.Context, _, _ string, _ time.Time) error {
	return f.activateErr
}
func (f *fakePairRepo) ListActive(_ context.Context) ([]domain.PairProposal, error) {
	return f.all, nil
}
func (f *fakePairRepo) ListAll(_ context.Context) ([]domain.PairProposal, error) {
	return f.all, nil
}

func TestPairService_Propose_Success(t *testing.T) {
	svc := NewPairService(&fakePairClient{proposeTx: "0xtx"}, &fakePairRepo{})
	res, err := svc.ProposePair(context.Background(), PairProposeRequest{PairID: "BRL-USD", TokenAAddress: "0xa", TokenBAddress: "0xb", AMMAddress: "0xamm", ProposerCB: "cb"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != domain.PairStatusProposed || res.TxHash != "0xtx" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestPairService_Propose_NilClient(t *testing.T) {
	svc := NewPairService(nil, &fakePairRepo{})
	if _, err := svc.ProposePair(context.Background(), PairProposeRequest{PairID: "X"}); err == nil {
		t.Fatal("expected nil-client error")
	}
}

func TestPairService_Propose_DuplicateByID(t *testing.T) {
	svc := NewPairService(&fakePairClient{}, &fakePairRepo{byID: &domain.PairProposal{Status: domain.PairStatusActive}})
	if _, err := svc.ProposePair(context.Background(), PairProposeRequest{PairID: "BRL-USD"}); !errors.Is(err, ErrPairAlreadyExists) {
		t.Fatalf("expected ErrPairAlreadyExists, got %v", err)
	}
}

func TestPairService_Propose_DuplicateByTokenPair(t *testing.T) {
	svc := NewPairService(&fakePairClient{}, &fakePairRepo{byTokenPair: &domain.PairProposal{Status: domain.PairStatusProposed}})
	if _, err := svc.ProposePair(context.Background(), PairProposeRequest{PairID: "BRL-USD"}); !errors.Is(err, ErrPairAlreadyExists) {
		t.Fatalf("expected ErrPairAlreadyExists, got %v", err)
	}
}

func TestPairService_Propose_OnChainErrorButAlreadyActive(t *testing.T) {
	svc := NewPairService(&fakePairClient{
		proposeErr: errors.New("revert"),
		active:     []domain.PairEntry{{PairID: "BRL-USD"}},
	}, &fakePairRepo{})
	if _, err := svc.ProposePair(context.Background(), PairProposeRequest{PairID: "BRL-USD"}); !errors.Is(err, ErrPairAlreadyExists) {
		t.Fatalf("expected ErrPairAlreadyExists, got %v", err)
	}
}

func TestPairService_Propose_OnChainErrorMapped(t *testing.T) {
	svc := NewPairService(&fakePairClient{proposeErr: errors.New("Unauthorized")}, &fakePairRepo{})
	if _, err := svc.ProposePair(context.Background(), PairProposeRequest{PairID: "X"}); !errors.Is(err, ErrNotCentralBankOfTokenA) {
		t.Fatalf("expected ErrNotCentralBankOfTokenA, got %v", err)
	}
}

func TestPairService_Confirm_Success(t *testing.T) {
	svc := NewPairService(&fakePairClient{confirmTx: "0xc"}, &fakePairRepo{})
	res, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "BRL-USD", ConfirmerCB: "cb-b"})
	if err != nil || res.Status != domain.PairStatusActive {
		t.Fatalf("unexpected: res=%+v err=%v", res, err)
	}
}

func TestPairService_Confirm_NilClient(t *testing.T) {
	if _, err := NewPairService(nil, &fakePairRepo{}).ConfirmPair(context.Background(), PairConfirmRequest{PairID: "X"}); err == nil {
		t.Fatal("expected nil-client error")
	}
}

func TestPairService_Confirm_IdempotentAlreadyActive(t *testing.T) {
	svc := NewPairService(&fakePairClient{
		confirmErr: errors.New("revert"),
		active:     []domain.PairEntry{{PairID: "BRL-USD", TokenA: "0xa", TokenB: "0xb"}},
	}, &fakePairRepo{activateErr: errors.New("not found")})
	res, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "BRL-USD", ConfirmerCB: "cb-b"})
	if err != nil || res.Status != domain.PairStatusActive {
		t.Fatalf("expected idempotent active, got res=%+v err=%v", res, err)
	}
}

func TestPairService_Confirm_OnChainErrorMapped(t *testing.T) {
	svc := NewPairService(&fakePairClient{confirmErr: errors.New("PAIR_NOT_FOUND")}, &fakePairRepo{})
	if _, err := svc.ConfirmPair(context.Background(), PairConfirmRequest{PairID: "X"}); !errors.Is(err, ErrPairNotFound) {
		t.Fatalf("expected ErrPairNotFound, got %v", err)
	}
}

func TestPairService_ListActivePairs_NoClient(t *testing.T) {
	repo := &fakePairRepo{all: []domain.PairProposal{{PairID: "BRL-USD", Status: domain.PairStatusActive}}}
	out, err := NewPairService(nil, repo).ListActivePairs(context.Background())
	if err != nil || len(out) != 1 {
		t.Fatalf("unexpected: out=%v err=%v", out, err)
	}
}

func TestPairService_ListActivePairs_SyncsPending(t *testing.T) {
	repo := &fakePairRepo{all: []domain.PairProposal{{PairID: "BRL-USD", Status: domain.PairStatusProposed}}}
	client := &fakePairClient{active: []domain.PairEntry{
		{PairID: "BRL-USD"},
		{PairID: "ARS-USD", TokenA: "0xa", TokenB: "0xb", AMMAddress: "0xamm"},
	}}
	out, err := NewPairService(client, repo).ListActivePairs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// BRL-USD synced to active + ARS-USD appended from chain.
	var foundActive, foundNew bool
	for _, p := range out {
		if p.PairID == "BRL-USD" && p.Status == domain.PairStatusActive {
			foundActive = true
		}
		if p.PairID == "ARS-USD" {
			foundNew = true
		}
	}
	if !foundActive || !foundNew {
		t.Fatalf("expected synced+appended pairs, got %+v", out)
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("error: AlreadyExists here", "AlreadyExists") {
		t.Fatal("expected match")
	}
	if containsAny("clean", "X", "Y") {
		t.Fatal("expected no match")
	}
}
