package server

import (
	"context"
	"errors"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
)

func TestLooksLikeKeycloakUserUUID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{" 550E8400-E29B-41D4-A716-446655440000 ", true}, // TrimSpace antes do match
		{"not-a-uuid", false},
		{"cbweb3-spoke-a-client", false},
		{"tryout-noc", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeKeycloakUserUUID(tc.in); got != tc.want {
			t.Errorf("looksLikeKeycloakUserUUID(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// mockKeycloakResolve implements keycloak.Client for resolveLoginUsernameForKeycloak tests.
type mockKeycloakResolve struct {
	adminToken string
	adminErr   error
	uname      string
	unameErr   error
}

func (m *mockKeycloakResolve) Login(ctx context.Context, username, password string) (keycloak.TokenResponse, error) {
	return keycloak.TokenResponse{}, errors.New("unexpected Login")
}
func (m *mockKeycloakResolve) Refresh(ctx context.Context, refreshToken string) (keycloak.TokenResponse, error) {
	return keycloak.TokenResponse{}, errors.New("unexpected Refresh")
}
func (m *mockKeycloakResolve) Logout(ctx context.Context, refreshToken string) error {
	return errors.New("unexpected Logout")
}
func (m *mockKeycloakResolve) ValidateToken(ctx context.Context, accessToken string) (domain.TokenClaims, error) {
	return domain.TokenClaims{}, errors.New("unexpected ValidateToken")
}
func (m *mockKeycloakResolve) CreateUser(ctx context.Context, adminToken string, user keycloak.CreateUserRequest) (string, error) {
	return "", errors.New("unexpected CreateUser")
}
func (m *mockKeycloakResolve) GetAdminToken(ctx context.Context) (string, error) {
	if m.adminErr != nil {
		return "", m.adminErr
	}
	return m.adminToken, nil
}
func (m *mockKeycloakResolve) UpdateUsername(ctx context.Context, adminToken, userID, newUsername string) error {
	return errors.New("unexpected UpdateUsername")
}
func (m *mockKeycloakResolve) ResetPassword(ctx context.Context, adminToken, userID, password string) error {
	return errors.New("unexpected ResetPassword")
}
func (m *mockKeycloakResolve) GetUserUsername(ctx context.Context, adminToken, userID string) (string, error) {
	if m.unameErr != nil {
		return "", m.unameErr
	}
	return m.uname, nil
}
func (m *mockKeycloakResolve) GetUserEmail(ctx context.Context, adminToken, userID string) (string, error) {
	return "", errors.New("unexpected GetUserEmail")
}
func (m *mockKeycloakResolve) AssignRealmRole(ctx context.Context, adminToken, userID, roleName string) error {
	return nil
}

func TestResolveLoginUsernameForKeycloak(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	uuid := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("non_uuid_passthrough", func(t *testing.T) {
		m := &mockKeycloakResolve{adminToken: "t", uname: "should-not-use"}
		got := resolveLoginUsernameForKeycloak(ctx, m, "tryout-noc")
		if got != "tryout-noc" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("uuid_resolves", func(t *testing.T) {
		m := &mockKeycloakResolve{adminToken: "adm", uname: "tryout-noc"}
		got := resolveLoginUsernameForKeycloak(ctx, m, uuid)
		if got != "tryout-noc" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("admin_token_error_keeps_uuid", func(t *testing.T) {
		m := &mockKeycloakResolve{adminErr: errors.New("no admin")}
		got := resolveLoginUsernameForKeycloak(ctx, m, uuid)
		if got != uuid {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("get_username_error_keeps_uuid", func(t *testing.T) {
		m := &mockKeycloakResolve{adminToken: "adm", unameErr: errors.New("404")}
		got := resolveLoginUsernameForKeycloak(ctx, m, uuid)
		if got != uuid {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("empty_resolved_username_keeps_uuid", func(t *testing.T) {
		m := &mockKeycloakResolve{adminToken: "adm", uname: "  "}
		got := resolveLoginUsernameForKeycloak(ctx, m, uuid)
		if got != uuid {
			t.Fatalf("got %q", got)
		}
	})
}
