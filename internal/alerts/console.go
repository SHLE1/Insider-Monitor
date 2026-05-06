package alerts

import (
	"fmt"
	"strings"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/utils"
)

// ConsoleAlerter implements basic console logging
type ConsoleAlerter struct{}

func (a *ConsoleAlerter) SendAlert(alert Alert) error {
	if alert.AlertType == "balance_change" {
		fmt.Println(formatConsoleAlphaAlert(alert))
		return nil
	}
	// Use colors based on alert level
	var color, symbol string
	switch alert.Level {
	case Critical:
		color = utils.ColorRed
		symbol = "🔴"
	case Warning:
		color = utils.ColorYellow
		symbol = "🟡"
	default:
		color = utils.ColorGreen
		symbol = "🟢"
	}

	// Format the timestamp
	timestamp := alert.Timestamp.Format("15:04:05")

	// Format alert type
	alertType := strings.ToUpper(alert.AlertType)
	if alertType == "BALANCE_CHANGE" {
		alertType = "BALANCE CHANGE"
	} else if alertType == "NEW_TOKEN" {
		alertType = "NEW TOKEN"
	} else if alertType == "NEW_WALLET" {
		alertType = "NEW WALLET"
	}

	// Draw a box around the alert
	width := 80
	topBorder := fmt.Sprintf("%s%s%s", color, strings.Repeat("━", width), utils.ColorReset)
	bottomBorder := topBorder

	// Print alert header
	fmt.Println(topBorder)
	fmt.Printf("%s%s [%s] %s ALERT - %s %s\n",
		color,
		symbol,
		timestamp,
		alertType,
		utils.ColorBold,
		utils.ColorReset)

	// Print alert details
	shortWallet := alert.WalletAddress
	if len(shortWallet) > 20 {
		shortWallet = shortWallet[:8] + "..." + shortWallet[len(shortWallet)-8:]
	}

	fmt.Printf("钱包：%s%s%s\n", utils.ColorBold, shortWallet, utils.ColorReset)
	if alert.ChainName != "" {
		fmt.Printf("链：%s%s%s\n", utils.ColorBold, alert.ChainName, utils.ColorReset)
	}

	// Format message content
	lines := strings.Split(alert.Message, "\n")
	for _, line := range lines {
		fmt.Println(line)
	}

	// Print any additional data if relevant
	if data, ok := alert.Data["change_percent"]; ok {
		if pct, ok := data.(float64); ok {
			direction := "↑"
			valueColor := utils.ColorGreen
			if pct < 0 {
				direction = "↓"
				valueColor = utils.ColorRed
			}
			fmt.Printf("变化：%s%s %.2f%%%s\n",
				valueColor,
				direction,
				pct,
				utils.ColorReset)
		}
	}

	fmt.Println(bottomBorder)

	return nil
}

func formatConsoleAlphaAlert(alert Alert) string {
	if alert.Data == nil {
		return alert.Message
	}
	symbol, _ := alert.Data["symbol"].(string)
	direction, _ := alert.Data["direction"].(string)
	amount, _ := alert.Data["amount_changed"].(uint64)
	decimals, _ := alert.Data["decimals"].(uint8)
	changePercent, _ := alert.Data["change_percent"].(float64)
	oldHoldingPct, _ := alert.Data["old_holding_pct"].(float64)
	newHoldingPct, _ := alert.Data["new_holding_pct"].(float64)
	fromAddress, _ := alert.Data["from_address"].(string)
	toAddress, _ := alert.Data["to_address"].(string)
	txHash, _ := alert.Data["tx_hash"].(string)
	status := "🟢"
	if direction == "流出" || changePercent < 0 {
		status = "🔴"
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
	return fmt.Sprintf("[TG] 代币名称 %s\n\n%s (UTC+8)\n\n代币检测 %s %s %s\n\n📊 initAmount %.4f%% | currentAmount %.4f%%\n\n变动总量 %s %.2f%% / %.2f%%\n\n发送地址：%s\n类型：%s\n\n接收地址：%s\n类型：%s\n\nToken：%s\n链：%s\nhashTx：%s",
		symbol,
		alert.Timestamp.In(time.FixedZone("UTC+8", 8*60*60)).Format("2006-01-02 15:04:05"),
		status,
		direction,
		utils.FormatTokenAmount(amount, decimals),
		oldHoldingPct,
		newHoldingPct,
		status,
		changePercent,
		absConsoleFloat(changePercent),
		fromAddress,
		consoleAddressType(fromAddress, alert.WalletAddress),
		toAddress,
		consoleAddressType(toAddress, alert.WalletAddress),
		alert.TokenMint,
		alert.ChainName,
		txHash,
	)
}

func consoleAddressType(address, watchedWallet string) string {
	if strings.EqualFold(address, watchedWallet) {
		return "监控钱包"
	}
	return "链上地址"
}

func absConsoleFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
