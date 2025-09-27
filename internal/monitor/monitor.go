package monitor

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/config"
	"github.com/accursedgalaxy/insider-monitor/internal/price"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// Constants for token decimals and display
const (
	DefaultTokenDecimals uint8 = 9
	MaxDisplayHoldings   int   = 5
)

// Terminal color codes for display
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// Display symbols
const (
	walletSymbol = "💼"
	tokenSymbol  = "🔹"
	dollarSymbol = "💲"
	moreSymbol   = "..."
	divider      = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
)

type WalletMonitor struct {
	client       *rpc.Client
	wallets      []solana.PublicKey
	networkURL   string
	isConnected  bool
	scanConfig   *config.ScanConfig
	priceService *price.JupiterPrice
}

func NewWalletMonitor(networkURL string, wallets []string, scanConfig *config.ScanConfig) (*WalletMonitor, error) {
	client := rpc.NewWithCustomRPCClient(rpc.NewWithLimiter(
		networkURL,
		4,
		1,
	))

	// Convert wallet addresses to PublicKeys
	pubKeys := make([]solana.PublicKey, len(wallets))
	for i, addr := range wallets {
		pubKey, err := solana.PublicKeyFromBase58(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid wallet address %s: %v", addr, err)
		}
		pubKeys[i] = pubKey
	}

	return &WalletMonitor{
		client:       client,
		wallets:      pubKeys,
		networkURL:   networkURL,
		scanConfig:   scanConfig,
		priceService: price.NewJupiterPrice(),
	}, nil
}

// Simplified TokenAccountInfo
type TokenAccountInfo struct {
	Balance         uint64    `json:"balance"`
	LastUpdated     time.Time `json:"last_updated"`
	Symbol          string    `json:"symbol"`
	Decimals        uint8     `json:"decimals"`
	USDPrice        float64   `json:"usd_price"`
	USDValue        float64   `json:"usd_value"`
	ConfidenceLevel string    `json:"confidence_level"`
}

// Simplified WalletData
type WalletData struct {
	WalletAddress string                      `json:"wallet_address"`
	TokenAccounts map[string]TokenAccountInfo `json:"token_accounts"` // mint -> info
	LastScanned   time.Time                   `json:"last_scanned"`
}

// Add these constants for retry configuration
const (
	maxRetries     = 5
	initialBackoff = 5 * time.Second
	maxBackoff     = 30 * time.Second
)

func (w *WalletMonitor) getTokenAccountsWithRetry(wallet solana.PublicKey) (*rpc.GetTokenAccountsResult, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		accounts, err := w.client.GetTokenAccountsByOwner(
			context.Background(),
			wallet,
			&rpc.GetTokenAccountsConfig{
				ProgramId: solana.TokenProgramID.ToPointer(),
			},
			&rpc.GetTokenAccountsOpts{
				Encoding: solana.EncodingBase64,
			},
		)

		if err == nil {
			return accounts, nil
		}

		lastErr = err
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too Many Requests") {
			log.Printf("⚠️  Rate limited on attempt %d for wallet %s, waiting %v before retry",
				attempt+1, wallet.String(), backoff)

			// Show helpful message on first rate limit
			if attempt == 0 {
				log.Printf("💡 Rate limit detected. This usually happens when using public RPC endpoints.")
				log.Printf("   Consider upgrading to a dedicated RPC provider:")
				log.Printf("   • Helius: 100k requests/day free - https://helius.dev")
				log.Printf("   • QuickNode: 30M requests/month free - https://quicknode.com")
				log.Printf("   • Triton: 10M requests/month free - https://triton.one")
			}

			time.Sleep(backoff)

			// Exponential backoff with max
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Handle other common errors with helpful messages
		if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("connection error: %w\n\n"+
				"💡 This might be due to:\n"+
				"   • Network connectivity issues\n"+
				"   • RPC endpoint is down or overloaded\n"+
				"   • Try a different RPC provider from the list above", err)
		}

		// If it's not a rate limit or connection error, return immediately
		return nil, fmt.Errorf("RPC request failed: %w\n\n"+
			"💡 If this error persists, try:\n"+
			"   • Check your RPC endpoint URL in config.json\n"+
			"   • Verify your network connection\n"+
			"   • Consider switching to a more reliable RPC provider", err)
	}

	// Enhanced final error message with actionable suggestions
	if strings.Contains(lastErr.Error(), "429") || strings.Contains(lastErr.Error(), "Too Many Requests") {
		return nil, fmt.Errorf("❌ Rate limit exceeded after %d retries\n\n"+
			"🔧 SOLUTION: You're likely using a public RPC endpoint with strict limits.\n"+
			"   Update your config.json with a dedicated RPC endpoint:\n\n"+
			"   {\n"+
			"     \"network_url\": \"YOUR_DEDICATED_RPC_URL_HERE\",\n"+
			"     ...\n"+
			"   }\n\n"+
			"🚀 Get a free RPC endpoint from:\n"+
			"   • Helius: https://helius.dev (100k requests/day)\n"+
			"   • QuickNode: https://quicknode.com (30M requests/month)\n"+
			"   • Triton: https://triton.one (10M requests/month)\n\n"+
			"Original error: %w", maxRetries, lastErr)
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// shouldIncludeToken determines if a token should be included based on scan configuration
func (w *WalletMonitor) shouldIncludeToken(mint string) bool {
	if w.scanConfig == nil {
		return true // If no scan config, include everything
	}

	switch w.scanConfig.ScanMode {
	case "whitelist":
		// Only include tokens in the IncludeTokens list
		for _, token := range w.scanConfig.IncludeTokens {
			if strings.EqualFold(token, mint) {
				return true
			}
		}
		return false

	case "blacklist":
		// Include all tokens except those in ExcludeTokens list
		for _, token := range w.scanConfig.ExcludeTokens {
			if strings.EqualFold(token, mint) {
				return false
			}
		}
		return true

	default: // "all" or any other value
		return true
	}
}

func (w *WalletMonitor) GetWalletData(wallet solana.PublicKey) (*WalletData, error) {
	walletData := &WalletData{
		WalletAddress: wallet.String(),
		TokenAccounts: make(map[string]TokenAccountInfo),
		LastScanned:   time.Now(),
	}

	// Use the retry version instead
	accounts, err := w.getTokenAccountsWithRetry(wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to get token accounts for wallet %s: %w", wallet.String(), err)
	}

	// Process token accounts
	for _, acc := range accounts.Value {
		var tokenAccount token.Account
		err = bin.NewBinDecoder(acc.Account.Data.GetBinary()).Decode(&tokenAccount)
		if err != nil {
			log.Printf("⚠️  Warning: failed to decode token account (this is usually normal): %v", err)
			continue
		}

		// Only include accounts with positive balance and that pass the filter
		if tokenAccount.Amount > 0 {
			mint := tokenAccount.Mint.String()
			if w.shouldIncludeToken(mint) {
				walletData.TokenAccounts[mint] = TokenAccountInfo{
					Balance:     tokenAccount.Amount,
					LastUpdated: time.Now(),
					Symbol:      mint[:8] + "...",
					Decimals:    DefaultTokenDecimals,
				}
			}
		}
	}

	log.Printf("✅ Wallet %s: found %d token accounts (after filtering)", wallet.String(), len(walletData.TokenAccounts))
	return walletData, nil
}

// Add these type definitions
type Change struct {
	WalletAddress string
	TokenMint     string
	TokenSymbol   string // Add symbol
	TokenDecimals uint8  // Add decimals
	ChangeType    string
	OldBalance    uint64
	NewBalance    uint64
	ChangePercent float64
	TokenBalances map[string]uint64 `json:",omitempty"`
}

func calculatePercentageChange(old, new uint64) float64 {
	if old == 0 {
		return 100.0 // Return 100% for new additions
	}

	// Convert to float64 before division to maintain precision
	oldFloat := float64(old)
	newFloat := float64(new)

	// Calculate percentage change
	change := ((newFloat - oldFloat) / oldFloat) * 100.0

	// Round to 2 decimal places to avoid floating point precision issues
	change = float64(int64(change*100)) / 100

	return change
}

// Utility function for absolute values
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func (w *WalletMonitor) checkConnection() error {
	// Try to get slot number as a simple connection test
	_, err := w.client.GetSlot(context.Background(), rpc.CommitmentFinalized)
	w.isConnected = err == nil

	if err != nil {
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too Many Requests") {
			return fmt.Errorf("RPC rate limit exceeded during connection check\n\n"+
				"💡 This indicates you're using a public RPC endpoint with strict limits.\n"+
				"   Consider upgrading to a dedicated RPC provider for reliable monitoring.\n\n"+
				"Original error: %w", err)
		}

		return fmt.Errorf("connection check failed: %w\n\n"+
			"💡 Troubleshooting steps:\n"+
			"   1. Check your network connection\n"+
			"   2. Verify your RPC endpoint URL in config.json\n"+
			"   3. Try a different RPC provider if the issue persists", err)
	}

	return nil
}

// Update ScanAllWallets to handle batches
func (w *WalletMonitor) ScanAllWallets() (map[string]*WalletData, error) {
	// Check connection first
	if err := w.checkConnection(); err != nil {
		return nil, err
	}

	results := make(map[string]*WalletData)
	batchSize := 2

	for i := 0; i < len(w.wallets); i += batchSize {
		end := i + batchSize
		if end > len(w.wallets) {
			end = len(w.wallets)
		}

		log.Printf("📊 Processing wallets %d-%d of %d", i+1, end, len(w.wallets))

		// Process batch
		for _, wallet := range w.wallets[i:end] {
			data, err := w.GetWalletData(wallet)
			if err != nil {
				log.Printf("❌ Error scanning wallet %s: %v", wallet.String(), err)
				// Return the error to propagate the enhanced error messages
				return nil, fmt.Errorf("failed to scan wallet %s: %w", wallet.String(), err)
			}
			results[wallet.String()] = data
		}

		// Small delay between batches to be nice to the RPC
		if end < len(w.wallets) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return results, nil
}

func DetectChanges(oldData, newData map[string]*WalletData, significantChange float64) []Change {
	var changes []Change

	// Check for changes in existing wallets
	for walletAddr, newWalletData := range newData {
		oldWalletData, existed := oldData[walletAddr]

		if !existed {
			continue // Skip new wallet detection for now
		}

		// Check for changes in existing wallet
		for mint, newInfo := range newWalletData.TokenAccounts {
			oldInfo, existed := oldWalletData.TokenAccounts[mint]

			if !existed {
				// New token detected
				changes = append(changes, Change{
					WalletAddress: walletAddr,
					TokenMint:     mint,
					TokenSymbol:   newInfo.Symbol,
					TokenDecimals: newInfo.Decimals,
					ChangeType:    "new_token",
					NewBalance:    newInfo.Balance,
				})
				continue
			}

			// Check for significant balance changes
			pctChange := calculatePercentageChange(oldInfo.Balance, newInfo.Balance)
			absChange := abs(pctChange)

			if absChange >= significantChange {
				changes = append(changes, Change{
					WalletAddress: walletAddr,
					TokenMint:     mint,
					TokenSymbol:   newInfo.Symbol,
					TokenDecimals: newInfo.Decimals,
					ChangeType:    "balance_change",
					OldBalance:    oldInfo.Balance,
					NewBalance:    newInfo.Balance,
					ChangePercent: pctChange,
				})
			}
		}
	}

	return changes
}

// Add this helper function
func formatTokenAmount(amount uint64, decimals uint8) string {
	if decimals == 0 {
		return fmt.Sprintf("%d", amount)
	}

	// Convert to float64 and divide by 10^decimals
	divisor := math.Pow(10, float64(decimals))
	value := float64(amount) / divisor

	// Format with appropriate decimal places based on size
	switch {
	case value >= 5000:
		return fmt.Sprintf("%.2fM", value/1000)
	case value >= 5:
		return fmt.Sprintf("%.2fK", value)
	default:
		return fmt.Sprintf("%.4f", value)
	}
}

// FormatWalletOverview returns a compact string representation of wallet holdings
func FormatWalletOverview(data map[string]*WalletData) string {
	var overview strings.Builder
	overview.WriteString("\nWallet Holdings Overview:\n")
	overview.WriteString("------------------------\n")

	for _, wallet := range data {
		overview.WriteString(fmt.Sprintf("📍 %s\n", wallet.WalletAddress))
		if len(wallet.TokenAccounts) == 0 {
			overview.WriteString("   No tokens found\n")
			continue
		}

		// Convert map to slice for sorting
		type tokenHolding struct {
			symbol   string
			balance  uint64
			decimals uint8
		}
		holdings := make([]tokenHolding, 0, len(wallet.TokenAccounts))
		for _, info := range wallet.TokenAccounts {
			holdings = append(holdings, tokenHolding{
				symbol:   info.Symbol,
				balance:  info.Balance,
				decimals: info.Decimals,
			})
		}

		// Sort by balance (highest first)
		sort.Slice(holdings, func(i, j int) bool {
			return holdings[i].balance > holdings[j].balance
		})

		// Show top 5 holdings
		maxDisplay := 5
		if len(holdings) < maxDisplay {
			maxDisplay = len(holdings)
		}
		for i := 0; i < maxDisplay; i++ {
			balance := formatTokenAmount(holdings[i].balance, holdings[i].decimals)
			overview.WriteString(fmt.Sprintf("   • %s: %s\n", holdings[i].symbol, balance))
		}

		// Show how many more tokens if any
		remaining := len(holdings) - maxDisplay
		if remaining > 0 {
			overview.WriteString(fmt.Sprintf("   ... and %d more tokens\n", remaining))
		}
		overview.WriteString("\n")
	}
	return overview.String()
}

// Update FormatWalletOverview to include confidence indicators
func formatTokenValue(value float64, confidence string) string {
	var indicator string
	switch strings.ToLower(confidence) {
	case "high":
		indicator = "✅"
	case "medium":
		indicator = "⚠️"
	default:
		indicator = "❓"
	}

	if value >= 1000000 {
		return fmt.Sprintf(" ($%.2fM) %s", value/1000000, indicator)
	} else if value >= 1000 {
		return fmt.Sprintf(" ($%.2fK) %s", value/1000, indicator)
	}
	return fmt.Sprintf(" ($%.2f) %s", value, indicator)
}

// TokenInfo holds complete token information including USD value
type TokenInfo struct {
	Mint     string
	Amount   float64 // Raw amount in smallest units
	USDValue float64
	Symbol   string
	Decimals uint8
}

// NewTokenInfo creates a TokenInfo from TokenAccountInfo and price data
func NewTokenInfo(mint string, accountInfo TokenAccountInfo, priceData price.PriceData, exists bool) TokenInfo {
	usdValue := 0.0
	if exists {
		actualAmount := float64(accountInfo.Balance) / math.Pow(10, float64(accountInfo.Decimals))
		usdValue = actualAmount * priceData.Price
	}

	symbol := accountInfo.Symbol
	if tokenName, found := getKnownTokenName(mint); found {
		symbol = tokenName
	}

	return TokenInfo{
		Mint:     mint,
		Amount:   float64(accountInfo.Balance),
		USDValue: usdValue,
		Symbol:   symbol,
		Decimals: accountInfo.Decimals,
	}
}

// GetDisplayAmount returns the formatted amount for display
func (t TokenInfo) GetDisplayAmount() string {
	actualAmount := t.Amount / math.Pow(10, float64(t.Decimals))

	switch {
	case actualAmount >= 1000000:
		return fmt.Sprintf("%.2fM", actualAmount/1000000)
	case actualAmount >= 1000:
		return fmt.Sprintf("%.2fK", actualAmount/1000)
	default:
		return fmt.Sprintf("%.4f", actualAmount)
	}
}

// GetValueColor returns the appropriate color for the USD value
func (t TokenInfo) GetValueColor() string {
	switch {
	case t.USDValue > 1000:
		return colorGreen
	case t.USDValue > 100:
		return colorCyan
	default:
		return colorWhite
	}
}

// updatePricesForWallets collects all unique token mints from wallets and updates their prices
func (m *WalletMonitor) updatePricesForWallets(walletDataMap map[string]*WalletData) {
	// Collect all unique mints
	mints := make([]string, 0)
	for _, walletData := range walletDataMap {
		for mint := range walletData.TokenAccounts {
			mints = append(mints, mint)
		}
	}

	// Update prices for all tokens
	if err := m.priceService.UpdatePrices(mints); err != nil {
		log.Printf("Error updating prices: %v", err)
	}
}

// processWalletHoldings converts wallet token accounts to TokenInfo objects and calculates total value
func (m *WalletMonitor) processWalletHoldings(walletData *WalletData) ([]TokenInfo, float64) {
	holdings := make([]TokenInfo, 0, len(walletData.TokenAccounts))
	walletTotalValue := 0.0

	for mint, info := range walletData.TokenAccounts {
		priceData, exists := m.priceService.GetPrice(mint)
		tokenInfo := NewTokenInfo(mint, info, priceData, exists)

		if exists {
			walletTotalValue += tokenInfo.USDValue
		}

		holdings = append(holdings, tokenInfo)
	}

	return holdings, walletTotalValue
}

// formatWalletValue formats a wallet's total value for display
func formatWalletValue(totalValue float64) string {
	if totalValue >= 1000000 {
		return fmt.Sprintf("$%.2fM", totalValue/1000000)
	} else if totalValue >= 1000 {
		return fmt.Sprintf("$%.2fK", totalValue/1000)
	} else {
		return fmt.Sprintf("$%.2f", totalValue)
	}
}

// formatPortfolioValue formats the total portfolio value for display
func formatPortfolioValue(totalValue float64) string {
	if totalValue >= 1000000 {
		return fmt.Sprintf("%s%sTOTAL PORTFOLIO VALUE: $%.2fM%s", colorBold, colorGreen, totalValue/1000000, colorReset)
	} else if totalValue >= 1000 {
		return fmt.Sprintf("%s%sTOTAL PORTFOLIO VALUE: $%.2fK%s", colorBold, colorGreen, totalValue/1000, colorReset)
	} else {
		return fmt.Sprintf("%s%sTOTAL PORTFOLIO VALUE: $%.2f%s", colorBold, colorGreen, totalValue, colorReset)
	}
}

// displayTokenHolding displays a single token holding
func displayTokenHolding(tokenInfo TokenInfo) {
	displayName := tokenInfo.Symbol
	if displayName == tokenInfo.Mint[:8]+"..." {
		if tokenName, found := getKnownTokenName(tokenInfo.Mint); found {
			displayName = tokenName
		}
	}

	amountStr := tokenInfo.GetDisplayAmount()
	valueColor := tokenInfo.GetValueColor()

	if tokenInfo.USDValue > 0 {
		fmt.Printf("   %s %s%-15s%s %12s %s%s($%.2f)%s\n",
			tokenSymbol,
			colorBold,
			displayName,
			colorReset,
			amountStr,
			valueColor,
			dollarSymbol,
			tokenInfo.USDValue,
			colorReset)
	} else {
		fmt.Printf("   %s %s%-15s%s %12s\n",
			tokenSymbol,
			colorBold,
			displayName,
			colorReset,
			amountStr)
	}
}

// DisplayWalletOverview displays a formatted overview of wallet holdings
func (m *WalletMonitor) DisplayWalletOverview(walletDataMap map[string]*WalletData) {
	fmt.Println()
	fmt.Printf("%s%s SOLANA WALLET MONITOR %s\n", colorBold, colorPurple, colorReset)
	fmt.Printf("%s%s %s\n\n", colorPurple, divider, colorReset)

	// Update prices for all tokens in all wallets
	m.updatePricesForWallets(walletDataMap)

	totalPortfolioValue := 0.0

	for _, wallet := range m.wallets {
		fmt.Printf("%s%s %s %s%s\n", colorBold, colorBlue, walletSymbol, wallet.String(), colorReset)

		walletData, exists := walletDataMap[wallet.String()]
		if !exists {
			fmt.Printf("   %sNo data available%s\n\n", colorYellow, colorReset)
			continue
		}

		// Process wallet holdings
		holdings, walletTotalValue := m.processWalletHoldings(walletData)
		totalPortfolioValue += walletTotalValue

		// Sort by USD value descending
		sort.Slice(holdings, func(i, j int) bool {
			return holdings[i].USDValue > holdings[j].USDValue
		})

		// Show wallet total
		if walletTotalValue > 0 {
			valueStr := formatWalletValue(walletTotalValue)
			fmt.Printf("   %s%sTotal Value: %s%s\n", colorBold, colorGreen, valueStr, colorReset)
		}

		// Display top holdings
		for i := 0; i < min(MaxDisplayHoldings, len(holdings)); i++ {
			displayTokenHolding(holdings[i])
		}

		if len(holdings) > MaxDisplayHoldings {
			fmt.Printf("   %s %s%d more tokens%s\n", moreSymbol, colorYellow, len(holdings)-MaxDisplayHoldings, colorReset)
		}
		fmt.Println()
	}

	// Display total portfolio value
	if totalPortfolioValue > 0 {
		fmt.Printf("%s%s %s\n", colorPurple, divider, colorReset)
		fmt.Println(formatPortfolioValue(totalPortfolioValue))
	}

	fmt.Printf("%s%s %s\n", colorPurple, divider, colorReset)
	fmt.Printf("%sLast updated: %s%s\n\n", colorYellow, time.Now().Format("2006-01-02 15:04:05"), colorReset)
}

// Helper function to lookup well-known token names
func getKnownTokenName(mint string) (string, bool) {
	// Map of well-known token mints to symbols
	knownTokens := map[string]string{
		"So11111111111111111111111111111111111111112":  "SOL",
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": "USDC",
		"Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB": "USDT",
		"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263": "BONK",
		"7dHbWXmci3dT8UFYWYZweBLXgycu7Y3iL6trKn1Y7ARj": "stSOL",
		"mSoLzYCxHdYgdzU16g5QSh3i5K3z3KZK7ytfqcJm7So":  "mSOL",
		"kinXdEcpDQeHPEuQnqmUgtYykqKGVFq6CeVX5iAHJq6":  "KIN",
		"JUPyiwrYJFskUPiHa7hkeR8VUtAeFoSYbKedZNsDvCN":  "JUP",
	}

	symbol, found := knownTokens[mint]
	return symbol, found
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
