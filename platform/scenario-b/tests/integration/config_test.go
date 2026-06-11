package integration_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config holds all environment-driven settings for the integration test suite.
// Defaults match the docker-compose port mappings and local dev credentials.
type Config struct {
	BankAURL        string
	BankBURL        string
	CentralBankAURL string
	CentralBankBURL string
	KeycloakURL     string

	BankARealm    string
	BankAClient   string
	BankASecret   string
	BankBRealm    string
	BankBClient   string
	BankBSecret   string
	CentralBankARealm   string
	CentralBankAClient  string
	CentralBankASecret  string
	CentralBankBRealm   string
	CentralBankBClient  string
	CentralBankBSecret  string

	SkipUp   bool
	SkipDown bool
}

func loadConfig() *Config {
	root := scenarioBRoot()
	return &Config{
		BankAURL:        envOr("API_GW_BANK_A_URL", "http://localhost:18080"),
		BankBURL:        envOr("API_GW_BANK_B_URL", "http://localhost:28080"),
		CentralBankAURL: envOr("API_GW_CENTRAL_BANK_A_URL", "http://localhost:38080"),
		CentralBankBURL: envOr("API_GW_CENTRAL_BANK_B_URL", "http://localhost:60080"),
		KeycloakURL:     envOr("KEYCLOAK_URL", "http://localhost:8081"),

		BankARealm:  envOr("KC_BANK_A_REALM", "bank-a"),
		BankAClient: envOr("KC_BANK_A_CLIENT", "bank-a-client"),
		BankASecret: envOrEnvFile("KC_BANK_A_SECRET",
			filepath.Join(root, "backend/config/.env.infra.bank-a"),
			"KC_CLIENT_SECRET", "bank-a-local-secret"),

		BankBRealm:  envOr("KC_BANK_B_REALM", "bank-b"),
		BankBClient: envOr("KC_BANK_B_CLIENT", "bank-b-client"),
		BankBSecret: envOrEnvFile("KC_BANK_B_SECRET",
			filepath.Join(root, "backend/config/.env.infra.bank-b"),
			"KC_CLIENT_SECRET", "bank-b-local-secret"),

		CentralBankARealm:  envOr("KC_CENTRAL_BANK_A_REALM", "central-bank-a"),
		CentralBankAClient: envOr("KC_CENTRAL_BANK_A_CLIENT", "central-bank-a-client"),
		CentralBankASecret: envOrEnvFile("KC_CENTRAL_BANK_A_SECRET",
			filepath.Join(root, "backend/config/.env.infra.central-bank-a"),
			"KC_CLIENT_SECRET", "central-bank-a-local-secret"),

		CentralBankBRealm:  envOr("KC_CENTRAL_BANK_B_REALM", "central-bank-b"),
		CentralBankBClient: envOr("KC_CENTRAL_BANK_B_CLIENT", "central-bank-b-client"),
		CentralBankBSecret: envOrEnvFile("KC_CENTRAL_BANK_B_SECRET",
			filepath.Join(root, "backend/config/.env.infra.central-bank-b"),
			"KC_CLIENT_SECRET", "central-bank-b-local-secret"),

		SkipUp:   os.Getenv("SKIP_UP") == "1",
		SkipDown: os.Getenv("SKIP_DOWN") != "0",
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrEnvFile returns os.Getenv(envKey) if set, otherwise reads fileKey from the
// .env file at filePath, otherwise returns fallback.
func envOrEnvFile(envKey, filePath, fileKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if v := readEnvFile(filePath, fileKey); v != "" {
		return v
	}
	return fallback
}

// readEnvFile parses a KEY=VALUE .env file and returns the value for the requested key.
func readEnvFile(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	prefix := key + "="
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

// scenarioBRoot returns the absolute path to scenario-b/ by walking up from
// this test file's location (tests/integration/ → scenario-b/).
func scenarioBRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "../.."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
