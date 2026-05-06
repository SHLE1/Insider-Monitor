package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/alerts"
	"github.com/accursedgalaxy/insider-monitor/internal/config"
	"github.com/accursedgalaxy/insider-monitor/internal/configui"
	"github.com/accursedgalaxy/insider-monitor/internal/monitor"
	"github.com/accursedgalaxy/insider-monitor/internal/storage"
	"github.com/accursedgalaxy/insider-monitor/internal/utils"
)

type WalletScanner interface {
	ScanAllWallets() (map[string]*monitor.WalletData, error)
	DisplayWalletOverview(walletDataMap map[string]*monitor.WalletData)
}

type ChangeEnricher interface {
	EnrichChanges(changes []monitor.Change) []monitor.Change
}

func main() {
	logger := utils.NewLogger(false)

	if len(os.Args) > 1 && os.Args[1] == "config-ui" {
		runConfigUI(logger, os.Args[2:])
		return
	}

	flags := flag.NewFlagSet("monitor", flag.ExitOnError)
	configPath := flags.String("config", "config.json", "Path to configuration file")
	_ = flags.Parse(os.Args[1:])

	fmt.Printf("\n%s%s 多链钱包监控 %s\n", utils.ColorBold, utils.ColorPurple, utils.ColorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", utils.ColorPurple, utils.ColorReset)

	cfg := loadAndValidateConfig(*configPath, logger)

	scanner, err := monitor.NewMultiChainMonitor(cfg.Chains)
	if err != nil {
		logger.Fatal("创建钱包监控失败：%v", err)
	}

	alerter := buildAlerter(cfg, logger)

	scanInterval, err := time.ParseDuration(cfg.ScanInterval)
	if err != nil {
		logger.Warning("扫描间隔 '%s' 无效，使用默认值 1 分钟", cfg.ScanInterval)
		scanInterval = time.Minute
	}

	runMonitor(scanner, alerter, cfg, scanInterval, logger)
}

func runConfigUI(logger *utils.Logger, args []string) {
	flags := flag.NewFlagSet("config-ui", flag.ExitOnError)
	configPath := flags.String("config", "config.json", "Path to configuration file")
	addr := flags.String("addr", "127.0.0.1:8787", "Local address for the config UI")
	_ = flags.Parse(args)

	logger.Config("配置页面已启动：http://%s", *addr)
	if err := configui.Serve(*addr, *configPath); err != nil {
		logger.Fatal("配置页面启动失败：%v", err)
	}
}

func loadAndValidateConfig(configPath string, logger *utils.Logger) *config.Config {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Fatal("找不到配置文件：%v\n\n处理方式：\n   1. 复制示例配置：cp config.example.json config.json\n   2. 修改 config.json\n   3. 在 .env 中填写 Solana 或 BSC RPC", err)
		}
		logger.Fatal("读取配置失败：%v", err)
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal("配置校验失败：\n%v", err)
	}
	return cfg
}

func buildAlerter(cfg *config.Config, logger *utils.Logger) alerts.Alerter {
	alerters := []alerts.Alerter{&alerts.ConsoleAlerter{}}
	logger.Config("终端提醒已启用")

	if cfg.Discord.Enabled {
		alerters = append(alerters, alerts.NewDiscordAlerter(cfg.Discord.WebhookURL, cfg.Discord.ChannelID))
		logger.Config("Discord 提醒已启用")
	}
	if cfg.Telegram.Enabled {
		alerters = append(alerters, alerts.NewTelegramAlerter(cfg.Telegram.BotToken, cfg.Telegram.ChatID))
		logger.Config("Telegram 提醒已启用")
	}

	return alerts.NewMultiAlerter(alerters...)
}

func runMonitor(scanner WalletScanner, alerter alerts.Alerter, cfg *config.Config, scanInterval time.Duration, logger *utils.Logger) {
	store := storage.New("./data")

	interrupt := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	var lastSuccessfulScan time.Time
	var connectionLost bool
	maxTimeBetweenScans := scanInterval * 3

	previousData, err := store.LoadWalletData()
	if err == nil {
		logger.Storage("已读取上次钱包数据")
	} else {
		logger.Warning("无法读取上次数据：%v。首次扫描后会重新建立基线。", err)
		previousData = make(map[string]*monitor.WalletData)
	}

	logger.Scan("开始首次钱包扫描...")
	initialResults, err := scanner.ScanAllWallets()
	if err != nil {
		logger.Error("首次扫描失败：%v", err)
		logger.Error("监控会继续在后台重试。")
	} else {
		if err := store.SaveWalletData(initialResults); err != nil {
			logger.Error("保存首次扫描数据失败：%v", err)
		}
		lastSuccessfulScan = time.Now()
		logger.Success("首次扫描完成，已获取 %d 个钱包的数据", len(initialResults))
		scanner.DisplayWalletOverview(initialResults)
	}

	go func() {
		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()

		logger.Info("监控循环已启动，扫描间隔：%v", scanInterval)

		for {
			select {
			case <-ticker.C:
				if !lastSuccessfulScan.IsZero() && time.Since(lastSuccessfulScan) > maxTimeBetweenScans && !connectionLost {
					connectionLost = true
					logger.Warning("%v 内没有成功扫描，已标记连接异常", maxTimeBetweenScans)
					continue
				}

				newResults, err := scanner.ScanAllWallets()
				if err != nil {
					logger.Error("扫描钱包失败：%v", err)
					if !connectionLost {
						connectionLost = true
						logger.Network("连接可能已中断，恢复前会暂停提醒，避免误报")
					}
					continue
				}

				if connectionLost {
					connectionLost = false
					logger.Network("连接已恢复，正在读取上次数据以避免误报")
					if savedData, err := store.LoadWalletData(); err == nil {
						previousData = savedData
					}
					lastSuccessfulScan = time.Now()
					continue
				}

				lastSuccessfulScan = time.Now()

				if len(previousData) > 0 {
					changes := monitor.DetectChanges(previousData, newResults, cfg.Alerts.SignificantChange)
					if enricher, ok := scanner.(ChangeEnricher); ok {
						changes = enricher.EnrichChanges(changes)
					}
					processChanges(changes, alerter, cfg.Alerts, logger)
				} else {
					logger.Info("首次扫描完成，已保存基线数据")
				}

				if err := store.SaveWalletData(newResults); err != nil {
					logger.Error("保存数据失败：%v", err)
				}
				previousData = newResults
				scanner.DisplayWalletOverview(newResults)

			case <-done:
				logger.Info("监控循环已停止")
				return
			}
		}
	}()

	<-interrupt
	logger.Info("正在安全退出...")
	if err := monitor.LogToFile("./data", "Monitor shutting down gracefully"); err != nil {
		logger.Error("写入退出日志失败：%v", err)
	}
	done <- true
	time.Sleep(time.Second)
}

func processChanges(changes []monitor.Change, alerter alerts.Alerter, alertCfg config.AlertConfig, logger *utils.Logger) {
	for _, change := range changes {
		var msg string
		var level alerts.AlertLevel
		var alertData map[string]interface{}

		switch change.ChangeType {
		case "new_wallet":
			var tokenDetails []string
			tokenData := make(map[string]uint64)
			tokenDecimals := make(map[string]uint8)
			for mint, balance := range change.TokenBalances {
				tokenDetails = append(tokenDetails, fmt.Sprintf("%s: %d", mint, balance))
				tokenData[mint] = balance
				tokenDecimals[mint] = monitor.DefaultTokenDecimals
			}
			msg = fmt.Sprintf("发现新钱包 %s，包含 %d 个资产：\n%s",
				change.WalletAddress,
				len(change.TokenBalances),
				strings.Join(tokenDetails, "\n"))
			level = alerts.Warning
			alertData = map[string]interface{}{
				"token_balances": tokenData,
				"token_decimals": tokenDecimals,
			}

		case "new_token":
			msg = fmt.Sprintf("钱包中发现新资产 %s (%s)，初始余额 %d",
				change.TokenSymbol, change.TokenMint, change.NewBalance)
			level = alerts.Warning
			alertData = map[string]interface{}{
				"balance":  change.NewBalance,
				"decimals": change.TokenDecimals,
				"symbol":   change.TokenSymbol,
			}

		case "balance_change":
			direction := change.Direction
			if direction == "" {
				direction = "变动"
			}
			msg = fmt.Sprintf("%s %s %d，余额 %d -> %d（%.2f%%）",
				change.TokenSymbol, direction, change.AmountChanged,
				change.OldBalance, change.NewBalance, change.ChangePercent)

			absChange := abs(change.ChangePercent)
			switch {
			case absChange >= (alertCfg.SignificantChange * 5):
				level = alerts.Critical
			case absChange >= (alertCfg.SignificantChange * 2):
				level = alerts.Warning
			default:
				level = alerts.Info
			}

			alertData = map[string]interface{}{
				"old_balance":     change.OldBalance,
				"new_balance":     change.NewBalance,
				"decimals":        change.TokenDecimals,
				"symbol":          change.TokenSymbol,
				"change_percent":  change.ChangePercent,
				"amount_changed":  change.AmountChanged,
				"direction":       direction,
				"old_holding_pct": change.OldHoldingPct,
				"new_holding_pct": change.NewHoldingPct,
				"tx_hash":         change.TxHash,
				"from_address":    change.FromAddress,
				"to_address":      change.ToAddress,
			}
		default:
			continue
		}

		if level == alerts.Warning || level == alerts.Critical {
			alert := alerts.Alert{
				Timestamp:     time.Now(),
				ChainName:     change.ChainName,
				ChainType:     change.ChainType,
				WalletAddress: change.WalletAddress,
				TokenMint:     change.TokenMint,
				AlertType:     change.ChangeType,
				Message:       msg,
				Level:         level,
				Data:          alertData,
			}

			if err := alerter.SendAlert(alert); err != nil {
				logger.Error("发送提醒失败：%v", err)
			}
		} else {
			logger.Info("%s", msg)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
