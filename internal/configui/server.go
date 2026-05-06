package configui

import (
	"encoding/json"
	"net/http"
	"os"

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

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return server.ListenAndServe()
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
