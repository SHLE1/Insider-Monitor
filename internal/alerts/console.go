package alerts

import (
	"fmt"
	"strings"

	"github.com/accursedgalaxy/insider-monitor/internal/utils"
)

// ConsoleAlerter implements basic console logging
type ConsoleAlerter struct{}

func (a *ConsoleAlerter) SendAlert(alert Alert) error {
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
