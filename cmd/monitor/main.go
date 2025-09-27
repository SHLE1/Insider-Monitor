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
	"github.com/accursedgalaxy/insider-monitor/internal/monitor"
	"github.com/accursedgalaxy/insider-monitor/internal/storage"
	"github.com/accursedgalaxy/insider-monitor/internal/utils"
)

// WalletScanner interface defines the contract for wallet monitoring
type WalletScanner interface {
	ScanAllWallets() (map[string]*monitor.WalletData, error)
	DisplayWalletOverview(walletDataMap map[string]*monitor.WalletData)
}

func main() {
	// Create our custom logger
	logger := utils.NewLogger(false)

	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Print welcome message
	fmt.Printf("\n%s%s SOLANA INSIDER MONITOR %s\n", utils.ColorBold, utils.ColorPurple, utils.ColorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", utils.ColorPurple, utils.ColorReset)

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Fatal("Configuration file not found: %v\n\n"+
				"💡 Quick fix:\n"+
				"   1. Copy the example: cp config.example.json config.json\n"+
				"   2. Edit config.json with your settings\n"+
				"   3. Get a free RPC endpoint from:\n"+
				"      • Helius: https://helius.dev\n"+
				"      • QuickNode: https://quicknode.com\n"+
				"      • Triton: https://triton.one", err)
		}
		logger.Fatal("Failed to load config: %v\n\n"+
			"💡 Check that your config.json file has valid JSON syntax.\n"+
			"   You can validate it at https://jsonlint.com/", err)
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal("Configuration validation failed:\n%v", err)
	}

	// Initialize scanner
	scanner, err := monitor.NewWalletMonitor(cfg.NetworkURL, cfg.Wallets, &cfg.Scan)
	if err != nil {
		logger.Fatal("Failed to create wallet monitor: %v\n\n"+
			"💡 This usually means:\n"+
			"   • Invalid wallet address format in config.json\n"+
			"   • Network connectivity issues\n"+
			"   • RPC endpoint problems\n\n"+
			"   Verify your wallet addresses are valid Solana addresses.", err)
	}

	// Initialize alerter
	var alerter alerts.Alerter
	if cfg.Discord.Enabled {
		alerter = alerts.NewDiscordAlerter(cfg.Discord.WebhookURL, cfg.Discord.ChannelID)
		logger.Config("Discord alerts enabled")
	} else {
		alerter = &alerts.ConsoleAlerter{}
		logger.Config("Console alerts enabled")
	}

	// Parse scan interval
	scanInterval, err := time.ParseDuration(cfg.ScanInterval)
	if err != nil {
		logger.Warning("Invalid scan interval '%s', using default of 1 minute", cfg.ScanInterval)
		scanInterval = time.Minute
	}

	runMonitor(scanner, alerter, cfg, scanInterval, logger)
}

func runMonitor(scanner WalletScanner, alerter alerts.Alerter, cfg *config.Config, scanInterval time.Duration, logger *utils.Logger) {
	storage := storage.New("./data")

	// Create buffered channels for graceful shutdown
	interrupt := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	// Track connection state
	var lastSuccessfulScan time.Time
	var connectionLost bool

	// Define maximum allowed time between successful scans
	maxTimeBetweenScans := scanInterval * 3

	// Initialize previousData from storage at startup
	var previousData map[string]*monitor.WalletData
	if savedData, err := storage.LoadWalletData(); err == nil {
		previousData = savedData
		logger.Storage("Loaded previous wallet data from storage")
	} else {
		logger.Warning("Could not load previous data: %v. Will initialize after first scan.", err)
		previousData = make(map[string]*monitor.WalletData)
	}

	// Perform initial scan immediately
	logger.Scan("Performing initial wallet scan...")
	initialResults, err := scanner.ScanAllWallets()
	if err != nil {
		logger.Error("Initial scan failed: %v", err)
		logger.Error("\n💡 Common solutions:")
		logger.Error("   • Check your internet connection")
		logger.Error("   • Verify your RPC endpoint is working")
		logger.Error("   • Ensure wallet addresses in config.json are valid")
		logger.Error("   • Try a different RPC provider if rate limited")
		logger.Error("\nThe monitor will continue trying in the background...")
	} else {
		if err := storage.SaveWalletData(initialResults); err != nil {
			logger.Error("Error saving initial data: %v", err)
		}
		lastSuccessfulScan = time.Now()
		logger.Success("Initial scan complete. Found data for %d wallets", len(initialResults))
		scanner.DisplayWalletOverview(initialResults)
	}

	// Start monitoring in a separate goroutine
	go func() {
		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()

		logger.Info("Starting monitoring loop with %v interval...", scanInterval)

		for {
			select {
			case <-ticker.C:
				// Check if we've exceeded the maximum time between scans
				if time.Since(lastSuccessfulScan) > maxTimeBetweenScans && !connectionLost {
					connectionLost = true
					logger.Warning("No successful scan in %v, marking connection as lost", maxTimeBetweenScans)
					continue
				}

				newResults, err := scanner.ScanAllWallets()
				if err != nil {
					logger.Error("Error scanning wallets: %v", err)
					if !connectionLost {
						connectionLost = true
						logger.Network("Connection appears to be lost, will suppress alerts until restored")
					}
					continue
				}

				// Connection restored check
				if connectionLost {
					connectionLost = false
					logger.Network("Connection restored, loading previous data to prevent false alerts")
					if savedData, err := storage.LoadWalletData(); err == nil {
						previousData = savedData
					}
					lastSuccessfulScan = time.Now()
					continue
				}

				// Update last successful scan time
				lastSuccessfulScan = time.Now()

				// Process changes only if we have previous data
				if len(previousData) > 0 {
					changes := monitor.DetectChanges(previousData, newResults, cfg.Alerts.SignificantChange)
					processChanges(changes, alerter, cfg.Alerts, logger)
				} else {
					// First scan, just store the data without generating alerts
					logger.Info("Initial scan completed, storing baseline data")
				}

				// Save new results
				if err := storage.SaveWalletData(newResults); err != nil {
					logger.Error("Error saving data: %v", err)
				}
				previousData = newResults

				// Display wallet overview
				scanner.DisplayWalletOverview(newResults)

			case <-done:
				logger.Info("Monitoring loop stopped")
				return
			}
		}
	}()

	// Wait for interrupt signal
	<-interrupt
	logger.Info("Shutting down gracefully...")
	if err := monitor.LogToFile("./data", "Monitor shutting down gracefully"); err != nil {
		logger.Error("Failed to write shutdown log: %v", err)
	}
	done <- true
	time.Sleep(time.Second) // Give a moment for final cleanup
}

func processChanges(changes []monitor.Change, alerter alerts.Alerter, alertCfg config.AlertConfig, logger *utils.Logger) {
	for _, change := range changes {
		var msg string
		var level alerts.AlertLevel
		var alertData map[string]interface{}

		switch change.ChangeType {
		case "new_wallet":
			// Create a consolidated message for all tokens
			var tokenDetails []string
			tokenData := make(map[string]uint64)
			tokenDecimals := make(map[string]uint8)
			for mint, balance := range change.TokenBalances {
				tokenDetails = append(tokenDetails, fmt.Sprintf("%s: %d", mint, balance))
				tokenData[mint] = balance
				tokenDecimals[mint] = monitor.DefaultTokenDecimals
			}
			msg = fmt.Sprintf("New wallet %s detected with %d tokens:\n%s",
				change.WalletAddress,
				len(change.TokenBalances),
				strings.Join(tokenDetails, "\n"))
			level = alerts.Warning
			alertData = map[string]interface{}{
				"token_balances": tokenData,
				"token_decimals": tokenDecimals,
			}

		case "new_token":
			msg = fmt.Sprintf("New token %s (%s) detected in wallet with initial balance %d",
				change.TokenSymbol, change.TokenMint, change.NewBalance)
			level = alerts.Warning
			alertData = map[string]interface{}{
				"balance":  change.NewBalance,
				"decimals": change.TokenDecimals,
				"symbol":   change.TokenSymbol,
			}

		case "balance_change":
			msg = fmt.Sprintf("Balance change for %s (%s): from %d to %d (%.2f%%)",
				change.TokenSymbol, change.TokenMint,
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
				"old_balance":    change.OldBalance,
				"new_balance":    change.NewBalance,
				"decimals":       change.TokenDecimals,
				"symbol":         change.TokenSymbol,
				"change_percent": change.ChangePercent,
			}
		}

		if level >= alerts.Warning {
			alert := alerts.Alert{
				Timestamp:     time.Now(),
				WalletAddress: change.WalletAddress,
				TokenMint:     change.TokenMint,
				AlertType:     change.ChangeType,
				Message:       msg,
				Level:         level,
				Data:          alertData,
			}

			if err := alerter.SendAlert(alert); err != nil {
				logger.Error("Failed to send alert: %v", err)
			}
		} else {
			logger.Info(msg)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
