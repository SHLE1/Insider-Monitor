package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/utils"
)

type TelegramAlerter struct {
	BotToken string
	ChatID   string
	client   *http.Client
}

type telegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func NewTelegramAlerter(botToken, chatID string) *TelegramAlerter {
	return &TelegramAlerter{
		BotToken: botToken,
		ChatID:   chatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TelegramAlerter) SendAlert(alert Alert) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	msg := telegramMessage{
		ChatID:    t.ChatID,
		Text:      formatTelegramAlert(alert),
		ParseMode: "Markdown",
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram message: %w", err)
	}

	resp, err := t.client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func formatTelegramAlert(alert Alert) string {
	if alert.AlertType == "balance_change" {
		return formatAlphaTelegramAlert(alert)
	}
	chain := alert.ChainName
	if chain == "" {
		chain = "Solana"
	}
	return fmt.Sprintf("*%s 提醒*\n链：`%s`\n钱包：`%s`\n资产：`%s`\n%s\n时间：`%s`",
		escapeMarkdown(strings.ToUpper(alert.AlertType)),
		escapeMarkdown(chain),
		escapeMarkdown(alert.WalletAddress),
		escapeMarkdown(alert.TokenMint),
		escapeMarkdown(alert.Message),
		alert.Timestamp.Format("2006-01-02 15:04:05 MST"),
	)
}

func formatAlphaTelegramAlert(alert Alert) string {
	symbol := dataString(alert.Data, "symbol", alert.TokenMint)
	direction := dataString(alert.Data, "direction", "变动")
	amount := dataUint64(alert.Data, "amount_changed")
	decimals := dataUint8(alert.Data, "decimals")
	changePercent := dataFloat64(alert.Data, "change_percent")
	oldHoldingPct := dataFloat64(alert.Data, "old_holding_pct")
	newHoldingPct := dataFloat64(alert.Data, "new_holding_pct")
	fromAddress := dataString(alert.Data, "from_address", "")
	toAddress := dataString(alert.Data, "to_address", "")
	txHash := dataString(alert.Data, "tx_hash", "")
	changed := "🟢"
	if direction == "流出" || changePercent < 0 {
		changed = "🔴"
	}

	if fromAddress == "" {
		fromAddress = alert.WalletAddress
	}
	if toAddress == "" {
		toAddress = alert.WalletAddress
	}
	if txHash == "" {
		txHash = "未匹配到最近交易"
	}

	return fmt.Sprintf("*[TG] 代币名称 %s*\n\n`%s (UTC+8)`\n\n代币检测 %s %s `%s`\n\n📊 initAmount `%.4f%%` | currentAmount `%.4f%%`\n\n变动总量 %s `%.2f%% / %.2f%%`\n\n发送地址：`%s`\n类型：%s\n\n接收地址：`%s`\n类型：%s\n\nToken：`%s`\n链：`%s`\n🔗 hashTx：`%s`",
		escapeMarkdown(symbol),
		alert.Timestamp.In(time.FixedZone("UTC+8", 8*60*60)).Format("2006-01-02 15:04:05"),
		changed,
		escapeMarkdown(direction),
		escapeMarkdown(utils.FormatTokenAmount(amount, decimals)),
		oldHoldingPct,
		newHoldingPct,
		changed,
		changePercent,
		absFloat(changePercent),
		escapeMarkdown(fromAddress),
		addressType(fromAddress, alert.WalletAddress),
		escapeMarkdown(toAddress),
		addressType(toAddress, alert.WalletAddress),
		escapeMarkdown(alert.TokenMint),
		escapeMarkdown(alert.ChainName),
		escapeMarkdown(txHash),
	)
}

func addressType(address, watchedWallet string) string {
	if strings.EqualFold(address, watchedWallet) {
		return "监控钱包"
	}
	return "链上地址"
}

func dataString(data map[string]interface{}, key, fallback string) string {
	if data == nil {
		return fallback
	}
	value, ok := data[key].(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func dataUint64(data map[string]interface{}, key string) uint64 {
	if data == nil {
		return 0
	}
	value, _ := data[key].(uint64)
	return value
}

func dataUint8(data map[string]interface{}, key string) uint8 {
	if data == nil {
		return 0
	}
	value, _ := data[key].(uint8)
	return value
}

func dataFloat64(data map[string]interface{}, key string) float64 {
	if data == nil {
		return 0
	}
	value, _ := data[key].(float64)
	return value
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func escapeMarkdown(value string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(value)
}
