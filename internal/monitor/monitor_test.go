package monitor

import (
	"testing"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/price"
	"github.com/stretchr/testify/assert"
)

func TestNewWalletMonitor(t *testing.T) {
	tests := []struct {
		name        string
		networkURL  string
		wallets     []string
		shouldError bool
	}{
		{
			name:       "Valid initialization",
			networkURL: "https://api.mainnet-beta.solana.com",
			wallets: []string{
				"DYw8jCTfwHNRJhhmFcbXvVDTqWMEVFBX6ZKUmG5CNSKK", // Example valid wallet
			},
			shouldError: false,
		},
		{
			name:       "Invalid wallet address",
			networkURL: "https://api.mainnet-beta.solana.com",
			wallets: []string{
				"invalid-address",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor, err := NewWalletMonitor(tt.networkURL, tt.wallets, nil)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, monitor)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, monitor)
				assert.Equal(t, tt.networkURL, monitor.networkURL)
				assert.Len(t, monitor.wallets, len(tt.wallets))
			}
		})
	}
}

func TestCalculatePercentageChange(t *testing.T) {
	tests := []struct {
		name     string
		old      uint64
		new      uint64
		expected float64
	}{
		{
			name:     "100% increase",
			old:      100,
			new:      200,
			expected: 100.0,
		},
		{
			name:     "50% decrease",
			old:      200,
			new:      100,
			expected: -50.0,
		},
		{
			name:     "New addition",
			old:      0,
			new:      100,
			expected: 100.0,
		},
		{
			name:     "No change",
			old:      100,
			new:      100,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePercentageChange(tt.old, tt.new)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectChanges(t *testing.T) {
	oldData := map[string]*WalletData{
		"wallet1": {
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{
				"token1": {
					Balance:     1000,
					LastUpdated: time.Now(),
					Symbol:      "TKN1",
					Decimals:    9,
				},
			},
		},
	}

	newData := map[string]*WalletData{
		"wallet1": {
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{
				"token1": {
					Balance:     2000,
					LastUpdated: time.Now(),
					Symbol:      "TKN1",
					Decimals:    9,
				},
			},
		},
	}

	changes := DetectChanges(oldData, newData, 50.0)
	assert.Len(t, changes, 1)
	assert.Equal(t, "wallet1", changes[0].WalletAddress)
	assert.Equal(t, "token1", changes[0].TokenMint)
	assert.Equal(t, uint64(1000), changes[0].OldBalance)
	assert.Equal(t, uint64(2000), changes[0].NewBalance)
	assert.Equal(t, 100.0, changes[0].ChangePercent)
}

func TestAbs(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "Positive number",
			input:    5.5,
			expected: 5.5,
		},
		{
			name:     "Negative number",
			input:    -5.5,
			expected: 5.5,
		},
		{
			name:     "Zero",
			input:    0.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := abs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTokenAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   uint64
		decimals uint8
		expected string
	}{
		{
			name:     "No decimals",
			amount:   1000,
			decimals: 0,
			expected: "1000",
		},
		{
			name:     "With decimals",
			amount:   1000000000,
			decimals: 9,
			expected: "1.0000",
		},
		{
			name:     "Millions",
			amount:   5000000000000,
			decimals: 9,
			expected: "5.00M",
		},
		{
			name:     "Thousands",
			amount:   5000000000,
			decimals: 9,
			expected: "5.00K",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTokenAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewTokenInfo(t *testing.T) {
	tokenAccountInfo := TokenAccountInfo{
		Balance:     1000000000, // 1 token with 9 decimals
		LastUpdated: time.Now(),
		Symbol:      "TKN1",
		Decimals:    9,
	}

	priceData := price.PriceData{
		Price:           2.0,
		LastUpdated:     time.Now(),
		ConfidenceLevel: "high",
	}

	tokenInfo := NewTokenInfo("mint1", tokenAccountInfo, priceData, true)

	assert.Equal(t, "mint1", tokenInfo.Mint)
	assert.Equal(t, float64(1000000000), tokenInfo.Amount)
	assert.Equal(t, 2.0, tokenInfo.USDValue) // 1 * 2.0
	assert.Equal(t, "TKN1", tokenInfo.Symbol)
	assert.Equal(t, uint8(9), tokenInfo.Decimals)
}

func TestTokenInfo_GetDisplayAmount(t *testing.T) {
	tests := []struct {
		name      string
		tokenInfo TokenInfo
		expected  string
	}{
		{
			name: "Small amount",
			tokenInfo: TokenInfo{
				Amount:   1000000000, // 1.0 with 9 decimals
				Decimals: 9,
			},
			expected: "1.0000",
		},
		{
			name: "Thousand",
			tokenInfo: TokenInfo{
				Amount:   1000000000000, // 1000.0 with 9 decimals -> displays as 1.00K
				Decimals: 9,
			},
			expected: "1.00K",
		},
		{
			name: "Million",
			tokenInfo: TokenInfo{
				Amount:   1000000000000000, // 1000000.0 with 9 decimals -> displays as 1.00M
				Decimals: 9,
			},
			expected: "1.00M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tokenInfo.GetDisplayAmount()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTokenInfo_GetValueColor(t *testing.T) {
	tests := []struct {
		name     string
		usdValue float64
		expected string
	}{
		{
			name:     "High value (>1000)",
			usdValue: 1500.0,
			expected: colorGreen,
		},
		{
			name:     "Medium value (>100)",
			usdValue: 500.0,
			expected: colorCyan,
		},
		{
			name:     "Low value",
			usdValue: 50.0,
			expected: colorWhite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenInfo := TokenInfo{USDValue: tt.usdValue}
			result := tokenInfo.GetValueColor()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatWalletValue(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{
			name:     "Small value",
			value:    123.45,
			expected: "$123.45",
		},
		{
			name:     "Thousand",
			value:    1234.56,
			expected: "$1.23K",
		},
		{
			name:     "Million",
			value:    1234567.89,
			expected: "$1.23M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWalletValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPortfolioValue(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{
			name:     "Small value",
			value:    123.45,
			expected: "\033[1m\033[32m总持仓价值：$123.45\033[0m",
		},
		{
			name:     "Thousand",
			value:    1234.56,
			expected: "\033[1m\033[32m总持仓价值：$1.23K\033[0m",
		},
		{
			name:     "Million",
			value:    1234567.89,
			expected: "\033[1m\033[32m总持仓价值：$1.23M\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPortfolioValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetKnownTokenName(t *testing.T) {
	tests := []struct {
		name     string
		mint     string
		expected string
		found    bool
	}{
		{
			name:     "Known token - SOL",
			mint:     "So11111111111111111111111111111111111111112",
			expected: "SOL",
			found:    true,
		},
		{
			name:     "Known token - USDC",
			mint:     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			expected: "USDC",
			found:    true,
		},
		{
			name:     "Unknown token",
			mint:     "unknown_mint_address",
			expected: "",
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := getKnownTokenName(tt.mint)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.found, found)
		})
	}
}
