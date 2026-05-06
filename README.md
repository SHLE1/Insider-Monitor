# Multi-Chain Insider Monitor

A tool for monitoring Solana and EVM wallet balances, with BSC support prioritized, balance-change alerts, local configuration UI, and Console/Discord/Telegram notifications.

## Community

Join our Discord community to:

- Get help with setup and configuration
- Share feedback and suggestions
- Connect with other users
- Stay updated on new features and releases
- Discuss Solana development

👉 [Join the Discord Server](https://discord.gg/7vY9ZBPdya)

## Features

- 🔍 Monitor multiple Solana and EVM/BSC wallets simultaneously
- 💰 Track native coin and token balance changes
- ⚡ Real-time alerts for significant changes
- 📊 EVM/BSC 代币余额变化支持 alpha 风格通知，包含流入/流出方向与持仓占比变化
- 🔔 Console, Discord, and Telegram notifications
- 🧭 Local browser-based configuration editor
- 🖥️ Web UI 可点击 Token 查看实时交易追踪，达到阈值时页面高亮并发送通知
- 💾 Persistent storage of wallet data
- 💵 Persist estimated USD price/value fields for scanned token holdings so the local dashboard can display portfolio value
- 🛡️ Graceful handling of network interruptions

---

## ⚠️ Important: RPC Endpoint Setup

**The most common setup issue is using the default public RPC endpoint**, which has strict rate limits and will cause scanning failures. Follow this guide to get a proper RPC endpoint:

### 🚀 Recommended RPC Providers (Free Tiers Available)

| Provider      | Free Tier          | Speed  | Setup                                     |
| ------------- | ------------------ | ------ | ----------------------------------------- |
| **Helius**    | 100k requests/day  | ⚡⚡⚡ | [Get Free Account](https://helius.dev)    |
| **QuickNode** | 30M requests/month | ⚡⚡⚡ | [Get Free Account](https://quicknode.com) |
| **Triton**    | 10M requests/month | ⚡⚡   | [Get Free Account](https://triton.one)    |
| **GenesysGo** | Custom limits      | ⚡⚡   | [Get Account](https://genesysgo.com)      |

### ❌ Avoid These (Rate Limited)

```
❌ https://api.mainnet-beta.solana.com (default - gets rate limited)
❌ https://api.devnet.solana.com (only for development)
❌ https://solana-api.projectserum.com (rate limited)
```

### ✅ How to Set Up Your RPC

1. **Sign up** for any provider above (they're free!)
2. **Get your RPC URL** from the dashboard
3. **Update your config.json**:
   ```json
   {
     "network_url": "https://your-custom-rpc-endpoint.com",
     ...
   }
   ```

---

## Quick Start

### Prerequisites

- Go 1.23.2 or later
- **A dedicated Solana RPC endpoint** (see [RPC Setup](#️-important-rpc-endpoint-setup) above - this is crucial!)

### Installation

```bash
# Clone the repository
git clone https://github.com/accursedgalaxy/insider-monitor
cd insider-monitor

# Install dependencies
go mod download
```

### Environment File

The monitor reads `.env` automatically from the project directory or the config file directory. Keep real RPC URLs and bot tokens there:

```bash
cp .env.example .env
```

Then edit `.env`:

```env
BSC_RPC_URL=https://your-bsc-rpc-endpoint
SOLANA_RPC_URL=https://your-solana-rpc-endpoint
DISCORD_WEBHOOK_URL=
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

### Configuration

1. Copy the example configuration:

```bash
cp config.example.json config.json
```

2. **⚠️ IMPORTANT**: Edit `config.json` and replace the RPC endpoints:

```json
{
  "scan_interval": "1m",
  "alerts": {
    "minimum_balance": 1000,
    "significant_change": 20,
    "ignore_tokens": []
  },
  "discord": {
    "enabled": false,
    "webhook_url": "",
    "channel_id": ""
  },
  "telegram": {
    "enabled": false,
    "bot_token": "${TELEGRAM_BOT_TOKEN}",
    "chat_id": ""
  },
  "chains": [
    {
      "type": "evm",
      "name": "BSC",
      "rpc_url": "${BSC_RPC_URL}",
      "chain_id": 56,
      "native_symbol": "BNB",
      "wallets": ["YOUR_EVM_WALLET_ADDRESS"],
      "scan": {
        "scan_mode": "whitelist",
        "tokens": [
          {
            "address": "0x55d398326f99059fF775485246999027B3197955",
            "symbol": "USDT",
            "decimals": 18
          }
        ]
      }
    }
  ]
}
```

3. **Get your RPC endpoint** from the [providers listed above](#-recommended-rpc-providers-free-tiers-available) and update `rpc_url`

> 本地配置和运行记录不会提交到 Git：`config.json`、`.env`、`data/`、`tmp/`、`*.log` 以及本地工具状态 `.pi-lens/` 已加入 `.gitignore`。如果你新增其他本地产生的文件，请同步加入 `.gitignore`，避免误提交密钥或运行数据。

### Configuration Options

- `chains`: Array of chains to monitor. Use `"type": "solana"` or `"type": "evm"`.
- `rpc_url`: Dedicated RPC endpoint URL. Secret values can use environment placeholders such as `${BSC_RPC_URL}`. The configuration UI masks this value by default and shows only “已配置” or “未配置”; click “修改” to edit it.
- `wallets`: Array of wallet addresses for the chain.
- `scan_interval`: Time between scans (e.g., "30s", "1m", "5m")
- `alerts`:
  - `minimum_balance`: Minimum token balance to trigger alerts
  - `significant_change`: Percentage change to trigger alerts (20 = 20%)
  - `ignore_tokens`: Array of token addresses to ignore
- `discord`:
  - `enabled`: Set to true to enable Discord notifications
  - `webhook_url`: Discord webhook URL
  - `channel_id`: Discord channel ID
- `telegram`:
  - `enabled`: Set to true to enable Telegram notifications
  - `bot_token`: Telegram bot token
  - `chat_id`: Telegram chat or channel ID
  - The configuration UI masks notification secrets by default. Configured values show as “已配置”; empty values show as “未配置”. Click “修改” to edit the actual value.
- `scan`:
  - `scan_mode`: Token scanning mode
    - `"all"`: Monitor all tokens (default)
    - `"whitelist"`: Only monitor tokens in `include_tokens`
    - `"blacklist"`: Monitor all tokens except those in `exclude_tokens`
  - `include_tokens`: Array of token addresses to specifically monitor (used with `whitelist` mode)
  - `exclude_tokens`: Array of token addresses to ignore (used with `blacklist` mode)
  - `tokens`: EVM/BSC token contracts to query. Standard RPC cannot reliably discover every BEP-20 token automatically, so BSC tokens should be listed here.

### Alpha 代币流向通知

对 `scan.tokens` 中配置的 EVM/BSC Token，监控会读取总供应量与被监控钱包余额。余额变化达到 `alerts.significant_change` 后，还会通过当前 RPC 查询最近的 ERC-20 `Transfer` 记录，尽量匹配造成这次余额变化的真实交易。Console、Telegram、Discord 通知会显示：

- Token 名称
- 流向：`流入` 或 `流出`
- 变动数量
- 初始持仓占比与当前持仓占比
- 变化百分比
- 匹配到链上记录时显示发送地址、接收地址与 `hashTx`
- 被监控钱包地址、Token 合约地址与链名称

该功能只使用已配置的 RPC。RPC URL、Telegram token、Discord webhook 仍放在 `.env`。为兼容 QuickNode Discover 等限制 `eth_getLogs` 查询范围的 RPC，默认只查询最近 5 个区块；如果交易不在最近查询范围内，通知仍会发送余额变化提醒，并标记为未匹配到最近交易。

在 Web UI 主界面点击某个 Token，界面会切换为左右两栏布局：左侧（约 40%）保持持仓列表，右侧（约 60%）展开实时追踪面板并吸附在视口内可独立滚动。追踪面板每 15 秒刷新一次该钱包在该 Token 上最近区块内的链上交易，并同步刷新余额与持仓占比；如果变化达到提醒阈值，面板会高亮显示，同时发送已启用的 Console、Telegram、Discord 通知。更长历史范围需要使用允许更大 `eth_getLogs` 区块范围的 RPC。

### Scan Mode Examples

Here are examples of different scan configurations:

1. Monitor all tokens:

```json
{
  "scan": {
    "scan_mode": "all",
    "include_tokens": [],
    "exclude_tokens": []
  }
}
```

2. Monitor only specific tokens (whitelist):

```json
{
  "scan": {
    "scan_mode": "whitelist",
    "include_tokens": [
      "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", // USDC
      "So11111111111111111111111111111111111111112" // SOL
    ],
    "exclude_tokens": []
  }
}
```

3. Monitor all tokens except specific ones (blacklist):

```json
{
  "scan": {
    "scan_mode": "blacklist",
    "include_tokens": [],
    "exclude_tokens": ["TokenAddressToIgnore1", "TokenAddressToIgnore2"]
  }
}
```

### Running the Monitor

```bash
go run cmd/monitor/main.go
```

Or build and run the current binary:

```bash
make build
./bin/insider-monitor -config config.json
```

### Local Configuration UI

```bash
go run cmd/monitor/main.go config-ui -config config.json
```

Then open `http://127.0.0.1:8787` in your browser. The UI now separates monitoring and configuration: the main page reads `data/wallet_data.json` and shows monitored addresses, current holdings, estimated USD value, and the latest scan time. Balance-change history is produced by comparing consecutive scans and sent through the configured alert channels. All editable settings live under the “配置” page, including chains, RPC, wallets, token contracts, thresholds, Discord, and Telegram settings. Wallet addresses and Token contract whitelist entries support multiple lines; use one address or one `address,symbol,decimals` Token entry per line. Environment-variable placeholders such as `${BSC_RPC_URL}` and `${TELEGRAM_BOT_TOKEN}` are only shown inside the configuration form, avoiding misleading values on the main monitoring page.

The configuration UI can trigger an on-demand scan from the main page. Click “立即扫描” to call `/api/scan`; the backend loads the current `config.json`, scans all configured chains/wallets, writes `data/wallet_data.json`, and returns the updated holdings to the dashboard.

Opening the page still does not start a background scan loop. To refresh holdings continuously, keep the monitor command running in another terminal. It performs an immediate first scan and then scans according to `scan_interval`:

```bash
go run cmd/monitor/main.go -config config.json
```

For EVM/BSC chains, native assets such as BNB are displayed alongside configured token contracts. Large BEP-20 balances are stored with an exact `raw_balance` string so the dashboard can display manually configured token balances and estimated USD values without `uint64` overflow. Short internal asset identifiers such as `native` are handled safely in terminal output.

### Development Mode (hot reload)

This project already supports AxonHub-style split development: Go backend uses `air` for hot reload, and the React/Vite frontend uses `pnpm dev`.

One-time setup:

```bash
go install github.com/air-verse/air@latest
make setup
```

Start two terminals from the project root:

```bash
# Terminal 1: backend API + config UI command, hot reload on Go changes
make dev-api
```

```bash
# Terminal 2: frontend Vite dev server, hot reload on UI changes
make dev-web
```

Open `http://127.0.0.1:5173`. In development, Vite proxies `/api` requests to the Go server on `http://127.0.0.1:8081`, and the backend runs with `INSIDER_DEV=1` so it only serves API/CORS instead of embedded static files.

Equivalent manual commands:

```bash
INSIDER_DEV=1 air
cd web
pnpm install
pnpm dev
```

`air` reads `.air.toml`, builds `./cmd/monitor`, and runs `config-ui --addr :8081 --config config.json` automatically.

#### Custom Config File

```bash
go run cmd/monitor/main.go -config path/to/config.json
```

### Alert Levels

The monitor uses three alert levels based on the configured `significant_change`:

- 🔴 **Critical**: Changes >= 5x the threshold
- 🟡 **Warning**: Changes >= 2x the threshold
- 🟢 **Info**: Changes below 2x the threshold

### Data Storage

The monitor stores wallet data in the `./data` directory to:

- Prevent false alerts after restarts
- Track historical changes
- Handle network interruptions gracefully

### Building from Source

```bash
make build
```

The binary will be available in the `bin` directory.

## 🔧 Troubleshooting

### Common Issues & Solutions

#### ❌ "Rate limit exceeded" / "Too Many Requests" Error

**Problem**: Using the default public RPC endpoint which has strict rate limits

```
❌ Rate limit exceeded after 5 retries
```

**Solution**:

1. Get a free RPC endpoint from [one of the providers above](#-recommended-rpc-providers-free-tiers-available)
2. Update your `config.json` with the new endpoint:
   ```json
   {
     "network_url": "https://your-custom-rpc-endpoint.com",
     ...
   }
   ```

#### ❌ "Invalid wallet address format" Error

**Problem**: Incorrect wallet address format in config.json

```
❌ invalid wallet address format at index 0: abc123
```

**Solution**: Ensure wallet addresses are valid Solana base58 encoded addresses (32-44 characters)

```json
{
  "wallets": [
    "CvQk2xkXtiMj2JqqVx1YZkeSqQ7jyQkNqqjeNE1jPTfc"  ✅ Valid format
  ]
}
```

#### ❌ "Configuration file not found" Error

**Problem**: config.json doesn't exist

```
❌ Configuration file not found: config.json
```

**Solution**:

```bash
cp config.example.json config.json
```

#### ❌ "Connection check failed" Error

**Problem**: Network or RPC endpoint issues

**Solution**:

1. Check your internet connection
2. Verify your RPC endpoint URL is correct
3. Try a different RPC provider
4. Test your RPC endpoint manually:
   ```bash
   curl -X POST -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"getSlot"}' \
     YOUR_RPC_ENDPOINT_URL
   ```

### Getting Help

If you're still having issues:

1. Check our [Discord community](https://discord.gg/7vY9ZBPdya) for help
2. Review the logs for specific error messages
3. Ensure you have the latest version of the monitor
4. Try the troubleshooting steps above

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
