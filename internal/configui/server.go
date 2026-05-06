package configui

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"

	"github.com/accursedgalaxy/insider-monitor/internal/config"
)

type pageData struct {
	ConfigJSON template.JS
}

func Serve(addr, configPath string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "请求方式不支持", http.StatusMethodNotAllowed)
			return
		}
		cfg, err := loadConfigForUI(configPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		payload, err := json.MarshalIndent(cfg.NormalizeForSave(), "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = pageTemplate.Execute(w, pageData{ConfigJSON: template.JS(payload)})
	})
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
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
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return server.ListenAndServe()
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
				RPCURL:       "${BSC_RPC_URL}",
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

var pageTemplate = template.Must(template.New("config").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Insider Monitor 配置</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #17202a;
      --muted: #627083;
      --line: #d9dee7;
      --accent: #1f7a5f;
      --danger: #b42318;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    main {
      width: min(1120px, calc(100vw - 32px));
      margin: 32px auto;
    }
    header {
      display: flex;
      align-items: flex-end;
      justify-content: space-between;
      gap: 20px;
      margin-bottom: 22px;
    }
    h1 { margin: 0; font-size: 30px; line-height: 1.1; }
    p { margin: 8px 0 0; color: var(--muted); }
    .grid {
      display: grid;
      grid-template-columns: 320px 1fr;
      gap: 18px;
      align-items: start;
    }
    section, aside {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 18px;
    }
    label {
      display: block;
      margin: 13px 0 6px;
      font-size: 13px;
      font-weight: 700;
      color: #273445;
    }
    input, select, textarea {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 6px;
      padding: 10px 11px;
      font: inherit;
      color: var(--text);
      background: #fff;
    }
    textarea {
      min-height: 96px;
      resize: vertical;
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 13px;
    }
    .row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 10px;
      margin-top: 16px;
    }
    button {
      border: 0;
      border-radius: 6px;
      padding: 10px 14px;
      font: inherit;
      font-weight: 700;
      cursor: pointer;
    }
    .primary { background: var(--accent); color: white; }
    .secondary { background: #e9edf3; color: #1f2937; }
    .danger { background: #fee4e2; color: var(--danger); }
    .chain {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 14px;
      margin: 12px 0;
      background: #fbfcfd;
    }
    .chain-title {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
    }
    .chain-title strong { font-size: 16px; }
    .status { min-height: 22px; margin-top: 12px; font-weight: 700; }
    .status.ok { color: var(--accent); }
    .status.err { color: var(--danger); }
    @media (max-width: 840px) {
      .grid { grid-template-columns: 1fr; }
      header { display: block; }
      .row { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
<main>
  <header>
    <div>
      <h1>Insider Monitor 配置</h1>
      <p>编辑链、钱包、Token 列表、提醒阈值和通知渠道。</p>
    </div>
    <button class="secondary" type="button" onclick="addBSC()">添加 BSC</button>
  </header>

  <div class="grid">
    <aside>
      <label>扫描间隔</label>
      <input id="scanInterval" placeholder="1m">
      <label>明显变化比例（百分比）</label>
      <input id="significantChange" type="number" step="0.01">
      <label>最低余额</label>
      <input id="minimumBalance" type="number" step="1">
      <label><input id="discordEnabled" type="checkbox" style="width:auto"> 启用 Discord</label>
      <input id="discordWebhook" placeholder="${DISCORD_WEBHOOK_URL}">
      <label><input id="telegramEnabled" type="checkbox" style="width:auto"> 启用 Telegram</label>
      <input id="telegramToken" placeholder="${TELEGRAM_BOT_TOKEN}">
      <label>Telegram Chat ID</label>
      <input id="telegramChat" placeholder="@channel_or_chat_id">
    </aside>

    <section>
      <div id="chains"></div>
      <div class="actions">
        <button class="secondary" type="button" onclick="render()">重置修改</button>
        <button class="primary" type="button" onclick="save()">保存配置</button>
      </div>
      <div id="status" class="status"></div>
    </section>
  </div>
</main>

<script>
let config = {{.ConfigJSON}};

function lines(value) {
  return String(value || "").split("\n").map(v => v.trim()).filter(Boolean);
}

function render() {
  document.getElementById("scanInterval").value = config.scan_interval || "1m";
  document.getElementById("significantChange").value = config.alerts?.significant_change ?? 20;
  document.getElementById("minimumBalance").value = config.alerts?.minimum_balance ?? 1000;
  document.getElementById("discordEnabled").checked = !!config.discord?.enabled;
  document.getElementById("discordWebhook").value = config.discord?.webhook_url || "";
  document.getElementById("telegramEnabled").checked = !!config.telegram?.enabled;
  document.getElementById("telegramToken").value = config.telegram?.bot_token || "";
  document.getElementById("telegramChat").value = config.telegram?.chat_id || "";

  const wrap = document.getElementById("chains");
  wrap.innerHTML = "";
  (config.chains || []).forEach((chain, index) => {
    const div = document.createElement("div");
    div.className = "chain";
    const selectedEVM = chain.type === "evm" ? "selected" : "";
    const selectedSolana = chain.type === "solana" ? "selected" : "";
    const wallets = (chain.wallets || []).join("\n");
    const tokens = ((chain.scan && chain.scan.tokens) || []).map(t => [t.address, t.symbol || "", t.decimals || ""].join(",")).join("\n");
    div.innerHTML =
      '<div class="chain-title">' +
      '<strong>' + (chain.name || "链") + '</strong>' +
      '<button class="danger" type="button" onclick="removeChain(' + index + ')">移除</button>' +
      '</div>' +
      '<div class="row">' +
      '<div><label>名称</label><input data-field="name" data-index="' + index + '" value="' + (chain.name || "") + '"></div>' +
      '<div><label>类型</label><select data-field="type" data-index="' + index + '">' +
      '<option value="evm" ' + selectedEVM + '>EVM</option>' +
      '<option value="solana" ' + selectedSolana + '>Solana</option>' +
      '</select></div>' +
      '</div>' +
      '<label>RPC URL</label><input data-field="rpc_url" data-index="' + index + '" value="' + (chain.rpc_url || "") + '">' +
      '<div class="row">' +
      '<div><label>Chain ID</label><input data-field="chain_id" data-index="' + index + '" type="number" value="' + (chain.chain_id || "") + '"></div>' +
      '<div><label>原生币符号</label><input data-field="native_symbol" data-index="' + index + '" value="' + (chain.native_symbol || "") + '"></div>' +
      '</div>' +
      '<label>钱包地址，每行一个</label>' +
      '<textarea data-field="wallets" data-index="' + index + '">' + wallets + '</textarea>' +
      '<label>Token 合约，每行一个：address,symbol,decimals</label>' +
      '<textarea data-field="tokens" data-index="' + index + '">' + tokens + '</textarea>';
    wrap.appendChild(div);
  });
}

function collect() {
  const next = {
    scan_interval: document.getElementById("scanInterval").value || "1m",
    alerts: {
      minimum_balance: Number(document.getElementById("minimumBalance").value || 0),
      significant_change: Number(document.getElementById("significantChange").value || 20),
      ignore_tokens: []
    },
    discord: {
      enabled: document.getElementById("discordEnabled").checked,
      webhook_url: document.getElementById("discordWebhook").value,
      channel_id: ""
    },
    telegram: {
      enabled: document.getElementById("telegramEnabled").checked,
      bot_token: document.getElementById("telegramToken").value,
      chat_id: document.getElementById("telegramChat").value
    },
    chains: JSON.parse(JSON.stringify(config.chains || []))
  };

  document.querySelectorAll("[data-field]").forEach(input => {
    const i = Number(input.dataset.index);
    const field = input.dataset.field;
    next.chains[i] = next.chains[i] || { scan: {} };
    if (field === "wallets") {
      next.chains[i].wallets = lines(input.value);
    } else if (field === "tokens") {
      next.chains[i].scan = next.chains[i].scan || {};
      next.chains[i].scan.scan_mode = "whitelist";
      next.chains[i].scan.tokens = lines(input.value).map(line => {
        const [address, symbol, decimals] = line.split(",").map(v => v.trim());
        return { address, symbol, decimals: Number(decimals || 0) };
      });
    } else if (field === "chain_id") {
      next.chains[i][field] = Number(input.value || 0);
    } else {
      next.chains[i][field] = input.value;
    }
  });
  return next;
}

function addBSC() {
  config.chains = config.chains || [];
  config.chains.push({
    type: "evm",
    name: "BSC",
    rpc_url: "${BSC_RPC_URL}",
    chain_id: 56,
    native_symbol: "BNB",
    wallets: [],
    scan: { scan_mode: "whitelist", tokens: [] }
  });
  render();
}

function removeChain(index) {
  config.chains.splice(index, 1);
  render();
}

async function save() {
  const status = document.getElementById("status");
  status.textContent = "";
  try {
    const next = collect();
    const res = await fetch("/api/config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(next)
    });
    if (!res.ok) throw new Error(await res.text());
    config = next;
    status.className = "status ok";
    status.textContent = "已保存。";
  } catch (err) {
    status.className = "status err";
    status.textContent = err.message;
  }
}

render();
</script>
</body>
</html>`))
