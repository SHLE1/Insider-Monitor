package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

func escapeMarkdown(value string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(value)
}
