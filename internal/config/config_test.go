package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLegacySolanaConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
		"network_url": "https://example.solana.rpc",
		"wallets": ["CvQk2xkXtiMj2JqqVx1YZkeSqQ7jyQkNqqjeNE1jPTfc"],
		"scan_interval": "1m",
		"alerts": {"minimum_balance": 1000, "significant_change": 0.20},
		"discord": {"enabled": false, "webhook_url": "", "channel_id": ""},
		"scan": {"scan_mode": "all", "include_tokens": [], "exclude_tokens": []}
	}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chains) != 1 {
		t.Fatalf("expected one migrated chain, got %d", len(cfg.Chains))
	}
	if cfg.Chains[0].Type != ChainTypeSolana {
		t.Fatalf("expected Solana chain, got %s", cfg.Chains[0].Type)
	}
	if cfg.Alerts.SignificantChange != 20 {
		t.Fatalf("expected legacy fractional threshold to become 20, got %v", cfg.Alerts.SignificantChange)
	}
}

func TestLoadMultiChainConfigWithEnv(t *testing.T) {
	t.Setenv("BSC_RPC_URL", "https://example.bsc.rpc")
	t.Setenv("TELEGRAM_BOT_TOKEN", "123:test")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
		"scan_interval": "30s",
		"alerts": {"minimum_balance": 1000, "significant_change": 20},
		"discord": {"enabled": false, "webhook_url": "", "channel_id": ""},
		"telegram": {"enabled": true, "bot_token": "${TELEGRAM_BOT_TOKEN}", "chat_id": "42"},
		"chains": [{
			"type": "evm",
			"name": "BSC",
			"rpc_url": "${BSC_RPC_URL}",
			"chain_id": 56,
			"native_symbol": "BNB",
			"wallets": ["0x1111111111111111111111111111111111111111"],
			"scan": {
				"scan_mode": "whitelist",
				"tokens": [{"address": "0x2222222222222222222222222222222222222222", "symbol": "TKN", "decimals": 18}]
			}
		}]
	}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Chains[0].RPCURL != "https://example.bsc.rpc" {
		t.Fatalf("expected env RPC URL, got %s", cfg.Chains[0].RPCURL)
	}
	if cfg.Telegram.BotToken != "123:test" {
		t.Fatalf("expected env Telegram token, got %s", cfg.Telegram.BotToken)
	}
}

func TestLoadConfigReadsDotEnvFile(t *testing.T) {
	t.Setenv("BSC_RPC_URL", "")
	_ = os.Unsetenv("BSC_RPC_URL")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("BSC_RPC_URL=https://from-dotenv.example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	body := `{
		"scan_interval": "30s",
		"alerts": {"minimum_balance": 1000, "significant_change": 20},
		"discord": {"enabled": false, "webhook_url": "", "channel_id": ""},
		"telegram": {"enabled": false, "bot_token": "", "chat_id": ""},
		"chains": [{
			"type": "evm",
			"name": "BSC",
			"rpc_url": "${BSC_RPC_URL}",
			"chain_id": 56,
			"native_symbol": "BNB",
			"wallets": ["0x1111111111111111111111111111111111111111"],
			"scan": {"scan_mode": "whitelist", "tokens": []}
		}]
	}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Chains[0].RPCURL != "https://from-dotenv.example" {
		t.Fatalf("expected .env RPC URL, got %s", cfg.Chains[0].RPCURL)
	}
}

func TestInvalidEVMAddress(t *testing.T) {
	cfg := Config{
		ScanInterval: "1m",
		Alerts:       AlertConfig{SignificantChange: 20},
		Chains: []ChainConfig{{
			Type:    ChainTypeEVM,
			Name:    "BSC",
			RPCURL:  "https://example.bsc.rpc",
			ChainID: 56,
			Wallets: []string{"not-an-address"},
		}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid EVM address error")
	}
}
