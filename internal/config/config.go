package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ChainTypeSolana = "solana"
	ChainTypeEVM    = "evm"
)

type Config struct {
	NetworkURL   string         `json:"network_url,omitempty"`
	Wallets      []string       `json:"wallets,omitempty"`
	ScanInterval string         `json:"scan_interval"`
	Alerts       AlertConfig    `json:"alerts"`
	Discord      DiscordConfig  `json:"discord"`
	Telegram     TelegramConfig `json:"telegram"`
	Scan         ScanConfig     `json:"scan,omitempty"`
	Chains       []ChainConfig  `json:"chains"`
}

type AlertConfig struct {
	MinimumBalance    uint64   `json:"minimum_balance"`
	SignificantChange float64  `json:"significant_change"`
	IgnoreTokens      []string `json:"ignore_tokens"`
}

type ScanConfig struct {
	IncludeTokens []string      `json:"include_tokens"`
	ExcludeTokens []string      `json:"exclude_tokens"`
	ScanMode      string        `json:"scan_mode"`
	Tokens        []TokenConfig `json:"tokens,omitempty"`
}

type TokenConfig struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol,omitempty"`
	Decimals uint8  `json:"decimals,omitempty"`
}

type ChainConfig struct {
	Type         string     `json:"type"`
	Name         string     `json:"name"`
	RPCURL       string     `json:"rpc_url"`
	ChainID      int64      `json:"chain_id,omitempty"`
	NativeSymbol string     `json:"native_symbol,omitempty"`
	Wallets      []string   `json:"wallets"`
	Scan         ScanConfig `json:"scan"`
}

type DiscordConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
	ChannelID  string `json:"channel_id"`
}

type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

var publicRPCEndpoints = []string{
	"https://api.mainnet-beta.solana.com",
	"https://api.devnet.solana.com",
	"https://api.testnet.solana.com",
	"https://solana-api.projectserum.com",
}

var recommendedRPCProviders = map[string]string{
	"Helius":    "100k requests/day free - https://helius.dev",
	"QuickNode": "30M requests/month free - https://quicknode.com",
	"Triton":    "10M requests/month free - https://triton.one",
	"GenesysGo": "Custom limits - https://genesysgo.com",
}

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func (c *Config) Validate() error {
	c.applyDefaults(true)

	if len(c.Chains) == 0 {
		return fmt.Errorf("至少需要配置一条链")
	}

	if c.ScanInterval == "" {
		c.ScanInterval = "1m"
	}
	if c.Alerts.SignificantChange == 0 {
		c.Alerts.SignificantChange = 20
	} else if c.Alerts.SignificantChange > 0 && c.Alerts.SignificantChange <= 1 {
		c.Alerts.SignificantChange *= 100
	}

	for i := range c.Chains {
		chain := &c.Chains[i]
		chain.applyDefaults()

		if chain.RPCURL == "" {
			return fmt.Errorf("链 %q 必须填写 RPC URL", chain.Name)
		}
		if len(chain.Wallets) == 0 {
			return fmt.Errorf("链 %q 至少需要一个钱包地址", chain.Name)
		}

		switch chain.Type {
		case ChainTypeSolana:
			if err := validateSolanaChain(chain); err != nil {
				return err
			}
			validateRPCEndpoint(chain.RPCURL)
		case ChainTypeEVM:
			if err := validateEVMChain(chain); err != nil {
				return err
			}
		default:
			return fmt.Errorf("链 %q 的类型 %q 暂不支持", chain.Name, chain.Type)
		}
	}

	if c.Discord.Enabled && c.Discord.WebhookURL == "" {
		return fmt.Errorf("启用 Discord 时必须填写 webhook_url")
	}
	if c.Telegram.Enabled {
		if c.Telegram.BotToken == "" {
			return fmt.Errorf("启用 Telegram 时必须填写 bot_token")
		}
		if c.Telegram.ChatID == "" {
			return fmt.Errorf("启用 Telegram 时必须填写 chat_id")
		}
	}

	return nil
}

func (c *Config) applyDefaults(resolveSecrets bool) {
	if resolveSecrets {
		c.Discord.WebhookURL = resolveValue(c.Discord.WebhookURL)
		c.Telegram.BotToken = resolveValue(c.Telegram.BotToken)
		c.Telegram.ChatID = resolveValue(c.Telegram.ChatID)
	}

	if len(c.Chains) == 0 && c.NetworkURL != "" {
		c.Chains = []ChainConfig{{
			Type:    ChainTypeSolana,
			Name:    "Solana",
			RPCURL:  c.NetworkURL,
			Wallets: c.Wallets,
			Scan:    c.Scan,
		}}
	}

	for i := range c.Chains {
		if resolveSecrets {
			c.Chains[i].RPCURL = resolveValue(c.Chains[i].RPCURL)
		}
	}
}

func (c *Config) NormalizeForSave() Config {
	out := *c
	out.NetworkURL = ""
	out.Wallets = nil
	out.Scan = ScanConfig{}
	for i := range out.Chains {
		out.Chains[i].applyDefaults()
	}
	return out
}

func (c *ChainConfig) applyDefaults() {
	c.Type = strings.ToLower(strings.TrimSpace(c.Type))
	if c.Type == "" {
		c.Type = ChainTypeSolana
	}
	if c.Name == "" {
		if c.Type == ChainTypeEVM && c.ChainID == 56 {
			c.Name = "BSC"
		} else {
			c.Name = strings.Title(c.Type)
		}
	}
	if c.Type == ChainTypeEVM {
		if c.NativeSymbol == "" {
			if c.ChainID == 56 {
				c.NativeSymbol = "BNB"
			} else {
				c.NativeSymbol = "ETH"
			}
		}
	}
	if c.Scan.ScanMode == "" {
		c.Scan.ScanMode = "all"
	}
}

func validateSolanaChain(chain *ChainConfig) error {
	for i, wallet := range chain.Wallets {
		if len(wallet) < 32 || len(wallet) > 44 {
			return fmt.Errorf("链 %q 第 %d 个 Solana 钱包地址无效：%s", chain.Name, i, wallet)
		}
	}
	return nil
}

func validateEVMChain(chain *ChainConfig) error {
	if chain.ChainID == 0 {
		return fmt.Errorf("EVM 链 %q 必须填写 chain_id", chain.Name)
	}
	for i, wallet := range chain.Wallets {
		if !evmAddressPattern.MatchString(wallet) {
			return fmt.Errorf("链 %q 第 %d 个 EVM 钱包地址无效：%s", chain.Name, i, wallet)
		}
	}
	for i, token := range chain.Scan.Tokens {
		if !evmAddressPattern.MatchString(token.Address) {
			return fmt.Errorf("链 %q 第 %d 个 EVM Token 地址无效：%s", chain.Name, i, token.Address)
		}
	}
	return nil
}

func validateRPCEndpoint(networkURL string) {
	for _, publicURL := range publicRPCEndpoints {
		if strings.EqualFold(networkURL, publicURL) {
			log.Printf("\n⚠️  WARNING: You're using a public RPC endpoint (%s)", networkURL)
			log.Printf("   Public endpoints have strict rate limits and may cause scanning issues.\n")
			log.Printf("🚀 RECOMMENDATION: Get a dedicated RPC endpoint for better performance:")
			for provider, details := range recommendedRPCProviders {
				log.Printf("   • %s: %s", provider, details)
			}
			log.Printf("\n💡 After getting your RPC endpoint, update your config file\n")
			return
		}
	}
	log.Printf("✅ 已使用自定义 RPC：%s", networkURL)
}

func resolveValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "env:") {
		return os.Getenv(strings.TrimPrefix(value, "env:"))
	}
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return os.Getenv(strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}"))
	}
	return os.ExpandEnv(value)
}

func LoadConfig(path string) (*Config, error) {
	loadDotEnvForConfig(path)

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败：%w", err)
	}
	cfg.applyDefaults(true)

	log.Printf("已读取配置：链数量=%d，扫描间隔=%s", len(cfg.Chains), cfg.ScanInterval)
	return &cfg, nil
}

func LoadConfigForEdit(path string) (*Config, error) {
	loadDotEnvForConfig(path)

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败：%w", err)
	}
	cfg.applyDefaults(false)
	return &cfg, nil
}

func loadDotEnvForConfig(configPath string) {
	_ = loadDotEnv(".env")
	if dir := filepath.Dir(configPath); dir != "." && dir != "" {
		_ = loadDotEnv(filepath.Join(dir, ".env"))
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}
