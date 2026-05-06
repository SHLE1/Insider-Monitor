package monitor

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/accursedgalaxy/insider-monitor/internal/config"
	"github.com/accursedgalaxy/insider-monitor/internal/price"
)

const nativeAssetAddress = "native"

type EVMMonitor struct {
	chain        config.ChainConfig
	httpClient   *http.Client
	priceService *price.DexScreenerPrice
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Result string          `json:"result"`
	Error  json.RawMessage `json:"error,omitempty"`
}

func NewEVMMonitor(chain config.ChainConfig) (*EVMMonitor, error) {
	return &EVMMonitor{
		chain: chain,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		priceService: price.NewDexScreenerPrice(),
	}, nil
}

func (m *EVMMonitor) ScanAllWallets() (map[string]*WalletData, error) {
	if err := m.checkConnection(); err != nil {
		return nil, err
	}

	results := make(map[string]*WalletData)
	for _, wallet := range m.chain.Wallets {
		data, err := m.GetWalletData(wallet)
		if err != nil {
			return nil, err
		}
		results[WalletDataKey(data.ChainName, data.WalletAddress)] = data
	}
	m.enrichPrices(results)
	return results, nil
}

func (m *EVMMonitor) GetWalletData(wallet string) (*WalletData, error) {
	walletData := &WalletData{
		ChainName:     m.chain.Name,
		ChainType:     config.ChainTypeEVM,
		WalletAddress: wallet,
		TokenAccounts: make(map[string]TokenAccountInfo),
		LastScanned:   time.Now(),
	}

	nativeBalance, err := m.getNativeBalance(wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to get native balance for %s: %w", wallet, err)
	}
	walletData.TokenAccounts[nativeAssetAddress] = TokenAccountInfo{
		Balance:     bigToUint64(nativeBalance),
		RawBalance:  nativeBalance.String(),
		LastUpdated: time.Now(),
		Symbol:      m.chain.NativeSymbol,
		Decimals:    18,
		Configured:  true,
	}

	for _, token := range m.chain.Scan.Tokens {
		info, err := m.getTokenInfo(wallet, token)
		if err != nil {
			return nil, fmt.Errorf("failed to get token %s for %s: %w", token.Address, wallet, err)
		}
		walletData.TokenAccounts[strings.ToLower(token.Address)] = info
	}

	return walletData, nil
}

func (m *EVMMonitor) DisplayWalletOverview(walletDataMap map[string]*WalletData) {
	fmt.Println()
	fmt.Printf("%s%s %s 钱包监控 %s\n", colorBold, colorPurple, strings.ToUpper(m.chain.Name), colorReset)
	fmt.Printf("%s%s %s\n\n", colorPurple, divider, colorReset)

	m.updatePrices(walletDataMap)

	for _, wallet := range m.chain.Wallets {
		fmt.Printf("%s%s %s %s%s\n", colorBold, colorBlue, walletSymbol, wallet, colorReset)
		walletData, exists := walletDataMap[WalletDataKey(m.chain.Name, wallet)]
		if !exists {
			fmt.Printf("   %s暂无数据%s\n\n", colorYellow, colorReset)
			continue
		}

		holdings := make([]TokenInfo, 0, len(walletData.TokenAccounts))
		for mint, info := range walletData.TokenAccounts {
			usdValue := 0.0
			if mint != nativeAssetAddress {
				if priceData, exists := m.priceService.GetPrice(mint); exists {
					actualAmount := tokenAmountFloat(info)
					usdValue = actualAmount * priceData.Price
				}
			}
			holdings = append(holdings, TokenInfo{
				Mint:     mint,
				Amount:   float64(info.Balance),
				USDValue: usdValue,
				Symbol:   info.Symbol,
				Decimals: info.Decimals,
			})
		}
		sort.Slice(holdings, func(i, j int) bool {
			return holdings[i].Amount > holdings[j].Amount
		})

		for i := 0; i < min(MaxDisplayHoldings, len(holdings)); i++ {
			displayTokenHolding(holdings[i])
		}
		if len(holdings) > MaxDisplayHoldings {
			fmt.Printf("   %s %s还有 %d 个 Token%s\n", moreSymbol, colorYellow, len(holdings)-MaxDisplayHoldings, colorReset)
		}
		fmt.Println()
	}

	fmt.Printf("%s%s %s\n", colorPurple, divider, colorReset)
	fmt.Printf("%s更新时间：%s%s\n\n", colorYellow, time.Now().Format("2006-01-02 15:04:05"), colorReset)
}

func (m *EVMMonitor) updatePrices(walletDataMap map[string]*WalletData) {
	if err := m.enrichPrices(walletDataMap); err != nil {
		fmt.Printf("%s提醒：更新 %s 价格失败：%v%s\n", colorYellow, m.chain.Name, err, colorReset)
	}
}

func (m *EVMMonitor) enrichPrices(walletDataMap map[string]*WalletData) error {
	addresses := make([]string, 0)
	for _, data := range walletDataMap {
		if data.ChainName != m.chain.Name {
			continue
		}
		for mint := range data.TokenAccounts {
			if mint != nativeAssetAddress {
				addresses = append(addresses, mint)
			}
		}
	}

	if err := m.priceService.UpdatePrices(addresses); err != nil {
		return err
	}

	for _, data := range walletDataMap {
		if data.ChainName != m.chain.Name {
			continue
		}
		for mint, info := range data.TokenAccounts {
			priceData, exists := m.priceService.GetPrice(mint)
			if !exists {
				continue
			}
			actualAmount := tokenAmountFloat(info)
			info.USDPrice = priceData.Price
			info.USDValue = actualAmount * priceData.Price
			info.ConfidenceLevel = priceData.ConfidenceLevel
			data.TokenAccounts[mint] = info
		}
	}
	return nil
}

func (m *EVMMonitor) checkConnection() error {
	result, err := m.rpcCall("eth_chainId", []interface{}{})
	if err != nil {
		return fmt.Errorf("connection check failed: %w", err)
	}
	chainID, err := hexToBigInt(result)
	if err != nil {
		return err
	}
	if m.chain.ChainID != 0 && chainID.Int64() != m.chain.ChainID {
		return fmt.Errorf("RPC chain_id mismatch for %s: expected %d, got %d", m.chain.Name, m.chain.ChainID, chainID.Int64())
	}
	return nil
}

func (m *EVMMonitor) getNativeBalance(wallet string) (*big.Int, error) {
	result, err := m.rpcCall("eth_getBalance", []interface{}{wallet, "latest"})
	if err != nil {
		return nil, err
	}
	return hexToBigInt(result)
}

func (m *EVMMonitor) getTokenInfo(wallet string, token config.TokenConfig) (TokenAccountInfo, error) {
	decimals := token.Decimals
	if decimals == 0 {
		value, err := m.callUint8(token.Address, "0x313ce567")
		if err == nil {
			decimals = value
		}
	}
	if decimals == 0 {
		decimals = 18
	}

	symbol := token.Symbol
	if symbol == "" {
		if value, err := m.callSymbol(token.Address); err == nil && value != "" {
			symbol = value
		}
	}
	if symbol == "" {
		symbol = token.Address[:8] + "..."
	}

	balance, err := m.callBalanceOf(token.Address, wallet)
	if err != nil {
		return TokenAccountInfo{}, err
	}

	return TokenAccountInfo{
		Balance:     bigToUint64(balance),
		RawBalance:  balance.String(),
		LastUpdated: time.Now(),
		Symbol:      symbol,
		Decimals:    decimals,
		Configured:  true,
	}, nil
}

func (m *EVMMonitor) callBalanceOf(contractAddress, wallet string) (*big.Int, error) {
	cleanWallet := strings.TrimPrefix(wallet, "0x")
	data := "0x70a08231" + strings.Repeat("0", 24) + cleanWallet
	result, err := m.ethCall(contractAddress, data)
	if err != nil {
		return nil, err
	}
	return hexToBigInt(result)
}

func (m *EVMMonitor) callUint8(contractAddress, data string) (uint8, error) {
	result, err := m.ethCall(contractAddress, data)
	if err != nil {
		return 0, err
	}
	value, err := hexToBigInt(result)
	if err != nil {
		return 0, err
	}
	return uint8(value.Uint64()), nil
}

func (m *EVMMonitor) callSymbol(contractAddress string) (string, error) {
	result, err := m.ethCall(contractAddress, "0x95d89b41")
	if err != nil {
		return "", err
	}
	return decodeABIString(result)
}

func (m *EVMMonitor) ethCall(to, data string) (string, error) {
	return m.rpcCall("eth_call", []interface{}{
		map[string]string{"to": to, "data": data},
		"latest",
	})
}

func (m *EVMMonitor) rpcCall(method string, params []interface{}) (string, error) {
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return "", err
	}

	resp, err := m.httpClient.Post(m.chain.RPCURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("RPC returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed rpcResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Error) > 0 && string(parsed.Error) != "null" {
		return "", fmt.Errorf("RPC error: %s", string(parsed.Error))
	}
	if parsed.Result == "" {
		return "", fmt.Errorf("RPC returned empty result")
	}
	return parsed.Result, nil
}

func hexToBigInt(value string) (*big.Int, error) {
	value = strings.TrimPrefix(value, "0x")
	if value == "" {
		return big.NewInt(0), nil
	}
	n := new(big.Int)
	if _, ok := n.SetString(value, 16); !ok {
		return nil, fmt.Errorf("invalid hex integer %q", value)
	}
	return n, nil
}

func bigToUint64(value *big.Int) uint64 {
	if value == nil || value.Sign() < 0 {
		return 0
	}
	if !value.IsUint64() {
		return math.MaxUint64
	}
	return value.Uint64()
}

func decodeABIString(hexValue string) (string, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(hexValue, "0x"))
	if err != nil {
		return "", err
	}
	if len(raw) == 32 {
		return strings.TrimRight(string(raw), "\x00"), nil
	}
	if len(raw) < 96 {
		return "", fmt.Errorf("invalid ABI string length")
	}
	length := new(big.Int).SetBytes(raw[32:64]).Int64()
	if length < 0 || int(64+length) > len(raw) {
		return "", fmt.Errorf("invalid ABI string payload")
	}
	return string(raw[64 : 64+length]), nil
}
