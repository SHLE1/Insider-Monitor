package configui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/alerts"
	"github.com/accursedgalaxy/insider-monitor/internal/config"
	"github.com/accursedgalaxy/insider-monitor/internal/monitor"
	"github.com/accursedgalaxy/insider-monitor/internal/storage"
)

func Serve(addr, configPath string) error {
	mux := http.NewServeMux()

	// In dev mode, Vite dev server serves the frontend; Go only needs CORS for API.
	if os.Getenv("INSIDER_DEV") != "" {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.NotFound(w, r)
		})
	} else if handler := staticHandler(); handler != nil {
		mux.Handle("/", handler)
	}

	mux.HandleFunc("/api/config", corsMiddleware(configHandler(configPath)))
	mux.HandleFunc("/api/wallet-data", corsMiddleware(walletDataHandler()))
	mux.HandleFunc("/api/scan", corsMiddleware(scanHandler(configPath)))
	mux.HandleFunc("/api/token-watch", corsMiddleware(tokenWatchHandler(configPath)))

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return server.ListenAndServe()
}

type tokenWatchResponse struct {
	WalletData map[string]*monitor.WalletData `json:"wallet_data"`
	Transfers  []monitor.TokenTransfer        `json:"transfers"`
	Alerts     []monitor.Change               `json:"alerts"`
}

func tokenWatchHandler(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "请求方式不支持", http.StatusMethodNotAllowed)
			return
		}

		chainName := strings.TrimSpace(r.URL.Query().Get("chain"))
		wallet := strings.TrimSpace(r.URL.Query().Get("wallet"))
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 50
		}
		if chainName == "" || wallet == "" || token == "" {
			http.Error(w, "必须提供 chain、wallet、token", http.StatusBadRequest)
			return
		}

		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			http.Error(w, "读取配置失败："+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := cfg.Validate(); err != nil {
			http.Error(w, "配置校验失败："+err.Error(), http.StatusBadRequest)
			return
		}

		chain, err := findEVMChain(cfg.Chains, chainName, wallet)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		evm, err := monitor.NewEVMMonitor(chain)
		if err != nil {
			http.Error(w, "创建 Token 追踪失败："+err.Error(), http.StatusInternalServerError)
			return
		}

		store := storage.New("./data")
		previousData, _ := store.LoadWalletData()
		newData, err := evm.ScanAllWallets()
		if err != nil {
			http.Error(w, "扫描钱包失败："+err.Error(), http.StatusBadGateway)
			return
		}

		changes := monitor.DetectChanges(previousData, newData, cfg.Alerts.SignificantChange)
		changes = evm.EnrichChanges(filterTokenChanges(changes, chain.Name, wallet, token))
		if len(changes) > 0 {
			sendWebUIAlerts(changes, cfg)
		}
		if err := store.SaveWalletData(mergeWalletData(previousData, newData)); err != nil {
			http.Error(w, "保存扫描结果失败："+err.Error(), http.StatusInternalServerError)
			return
		}

		transfers, err := evm.RecentTokenTransfers(wallet, token, limit)
		if err != nil {
			http.Error(w, "读取链上交易失败："+err.Error(), http.StatusBadGateway)
			return
		}

		writeJSON(w, tokenWatchResponse{
			WalletData: newData,
			Transfers:  transfers,
			Alerts:     changes,
		})
	}
}

func findEVMChain(chains []config.ChainConfig, chainName, wallet string) (config.ChainConfig, error) {
	for _, chain := range chains {
		if chain.Type != config.ChainTypeEVM || !strings.EqualFold(chain.Name, chainName) {
			continue
		}
		for _, configuredWallet := range chain.Wallets {
			if strings.EqualFold(configuredWallet, wallet) {
				return chain, nil
			}
		}
	}
	return config.ChainConfig{}, fmt.Errorf("未找到匹配的 EVM 链和钱包")
}

func filterTokenChanges(changes []monitor.Change, chainName, wallet, token string) []monitor.Change {
	filtered := make([]monitor.Change, 0, len(changes))
	for _, change := range changes {
		if strings.EqualFold(change.ChainName, chainName) &&
			strings.EqualFold(change.WalletAddress, wallet) &&
			strings.EqualFold(change.TokenMint, token) {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

func mergeWalletData(base, updates map[string]*monitor.WalletData) map[string]*monitor.WalletData {
	if base == nil {
		base = make(map[string]*monitor.WalletData)
	}
	for key, value := range updates {
		base[key] = value
	}
	return base
}

func sendWebUIAlerts(changes []monitor.Change, cfg *config.Config) {
	alerters := []alerts.Alerter{&alerts.ConsoleAlerter{}}
	if cfg.Discord.Enabled {
		alerters = append(alerters, alerts.NewDiscordAlerter(cfg.Discord.WebhookURL, cfg.Discord.ChannelID))
	}
	if cfg.Telegram.Enabled {
		alerters = append(alerters, alerts.NewTelegramAlerter(cfg.Telegram.BotToken, cfg.Telegram.ChatID))
	}
	alerter := alerts.NewMultiAlerter(alerters...)
	for _, change := range changes {
		alert := alerts.Alert{
			Timestamp:     time.Now(),
			ChainName:     change.ChainName,
			ChainType:     change.ChainType,
			WalletAddress: change.WalletAddress,
			TokenMint:     change.TokenMint,
			AlertType:     change.ChangeType,
			Message:       fmt.Sprintf("%s %s %d，余额 %d -> %d（%.2f%%）", change.TokenSymbol, change.Direction, change.AmountChanged, change.OldBalance, change.NewBalance, change.ChangePercent),
			Level:         alertLevelForChange(change, cfg.Alerts.SignificantChange),
			Data: map[string]interface{}{
				"old_balance":     change.OldBalance,
				"new_balance":     change.NewBalance,
				"decimals":        change.TokenDecimals,
				"symbol":          change.TokenSymbol,
				"change_percent":  change.ChangePercent,
				"amount_changed":  change.AmountChanged,
				"direction":       change.Direction,
				"old_holding_pct": change.OldHoldingPct,
				"new_holding_pct": change.NewHoldingPct,
				"tx_hash":         change.TxHash,
				"from_address":    change.FromAddress,
				"to_address":      change.ToAddress,
			},
		}
		_ = alerter.SendAlert(alert)
	}
}

func alertLevelForChange(change monitor.Change, threshold float64) alerts.AlertLevel {
	changePercent := change.ChangePercent
	if changePercent < 0 {
		changePercent = -changePercent
	}
	switch {
	case changePercent >= threshold*5:
		return alerts.Critical
	case changePercent >= threshold*2:
		return alerts.Warning
	default:
		return alerts.Info
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("INSIDER_DEV") != "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func configHandler(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, err := loadConfigForUI(configPath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, cfg.NormalizeForSave())
		case http.MethodPost:
			var cfg config.Config
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "配置内容不是有效 JSON："+err.Error(), http.StatusBadRequest)
				return
			}
			payload, err := json.MarshalIndent(cfg.NormalizeForSave(), "", "  ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := os.WriteFile(configPath, append(payload, '\n'), 0644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]string{"status": "saved"})
		default:
			http.Error(w, "请求方式不支持", http.StatusMethodNotAllowed)
		}
	}
}

func walletDataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "请求方式不支持", http.StatusMethodNotAllowed)
			return
		}

		data, err := storage.New("./data").LoadWalletData()
		if err != nil {
			http.Error(w, "读取钱包数据失败："+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, data)
	}
}

func scanHandler(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "请求方式不支持", http.StatusMethodNotAllowed)
			return
		}

		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			http.Error(w, "读取配置失败："+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := cfg.Validate(); err != nil {
			http.Error(w, "配置校验失败："+err.Error(), http.StatusBadRequest)
			return
		}

		scanner, err := monitor.NewMultiChainMonitor(cfg.Chains)
		if err != nil {
			http.Error(w, "创建钱包监控失败："+err.Error(), http.StatusInternalServerError)
			return
		}

		data, err := scanner.ScanAllWallets()
		if err != nil {
			http.Error(w, "扫描钱包失败："+err.Error(), http.StatusBadGateway)
			return
		}

		if err := storage.New("./data").SaveWalletData(data); err != nil {
			http.Error(w, "保存扫描结果失败："+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, data)
	}
}

func loadConfigForUI(configPath string) (*config.Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return defaultConfig(), nil
	}
	cfg, err := config.LoadConfigForEdit(configPath)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfig() *config.Config {
	return &config.Config{
		ScanInterval: "1m",
		Alerts: config.AlertConfig{
			MinimumBalance:    1000,
			SignificantChange: 20,
		},
		Discord:  config.DiscordConfig{},
		Telegram: config.TelegramConfig{},
		Chains: []config.ChainConfig{
			{
				Type:         config.ChainTypeEVM,
				Name:         "BSC",
				RPCURL:       "",
				ChainID:      56,
				NativeSymbol: "BNB",
				Wallets:      []string{},
				Scan: config.ScanConfig{
					ScanMode: "whitelist",
					Tokens:   []config.TokenConfig{},
				},
			},
		},
	}
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
