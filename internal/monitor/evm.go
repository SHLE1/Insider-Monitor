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

type rpcRawResponse struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type evmLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	TransactionHash  string   `json:"transactionHash"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionIndex string   `json:"transactionIndex"`
	LogIndex         string   `json:"logIndex"`
}

type transferMatch struct {
	TxHash string
	From   string
	To     string
	Amount uint64
}

type TokenTransfer struct {
	TxHash      string `json:"tx_hash"`
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      uint64 `json:"amount"`
	RawAmount   string `json:"raw_amount"`
	Direction   string `json:"direction"`
	BlockNumber uint64 `json:"block_number"`
	LogIndex    uint64 `json:"log_index"`
}

const (
	erc20TransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	defaultLogLookback = uint64(5)
)

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

func (m *EVMMonitor) EnrichChanges(changes []Change) []Change {
	for i := range changes {
		change := &changes[i]
		if change.ChainName != m.chain.Name || change.ChainType != config.ChainTypeEVM {
			continue
		}
		if change.ChangeType != "balance_change" || change.TokenMint == nativeAssetAddress {
			continue
		}
		match, err := m.findRecentTransfer(*change)
		if err != nil {
			continue
		}
		change.TxHash = match.TxHash
		change.FromAddress = match.From
		change.ToAddress = match.To
	}
	return changes
}

func (m *EVMMonitor) RecentTokenTransfers(walletAddress, tokenAddress string, limit int) ([]TokenTransfer, error) {
	if limit <= 0 {
		limit = 50
	}
	currentBlock, err := m.currentBlockNumber()
	if err != nil {
		return nil, err
	}
	fromBlock := uint64(0)
	if currentBlock > defaultLogLookback {
		fromBlock = currentBlock - defaultLogLookback
	}

	walletTopic := addressToTopic(walletAddress)
	inLogs, inErr := m.getTransferLogs(tokenAddress, fromBlock, currentBlock, []interface{}{erc20TransferTopic, nil, walletTopic})
	outLogs, outErr := m.getTransferLogs(tokenAddress, fromBlock, currentBlock, []interface{}{erc20TransferTopic, walletTopic})
	if inErr != nil && outErr != nil {
		return nil, inErr
	}

	transfers := make([]TokenTransfer, 0, len(inLogs)+len(outLogs))
	for _, log := range inLogs {
		if transfer, ok := tokenTransferFromLog(log, walletAddress); ok {
			transfers = append(transfers, transfer)
		}
	}
	for _, log := range outLogs {
		if transfer, ok := tokenTransferFromLog(log, walletAddress); ok {
			transfers = append(transfers, transfer)
		}
	}
	sort.Slice(transfers, func(i, j int) bool {
		if transfers[i].BlockNumber == transfers[j].BlockNumber {
			return transfers[i].LogIndex > transfers[j].LogIndex
		}
		return transfers[i].BlockNumber > transfers[j].BlockNumber
	})
	if len(transfers) > limit {
		transfers = transfers[:limit]
	}
	return transfers, nil
}

func tokenTransferFromLog(log evmLog, walletAddress string) (TokenTransfer, bool) {
	match, ok := transferFromLog(log)
	if !ok {
		return TokenTransfer{}, false
	}
	amount, err := hexToBigInt(log.Data)
	if err != nil {
		return TokenTransfer{}, false
	}
	blockNumber, _ := hexToBigInt(log.BlockNumber)
	logIndex, _ := hexToBigInt(log.LogIndex)
	direction := "流入"
	if strings.EqualFold(match.From, walletAddress) {
		direction = "流出"
	}
	return TokenTransfer{
		TxHash:      match.TxHash,
		From:        match.From,
		To:          match.To,
		Amount:      match.Amount,
		RawAmount:   amount.String(),
		Direction:   direction,
		BlockNumber: blockNumber.Uint64(),
		LogIndex:    logIndex.Uint64(),
	}, true
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
	totalSupply, _ := m.callTotalSupply(token.Address)

	return TokenAccountInfo{
		Balance:        bigToUint64(balance),
		RawBalance:     balance.String(),
		TotalSupplyRaw: totalSupply.String(),
		HoldingPercent: holdingPercent(balance, totalSupply),
		LastUpdated:    time.Now(),
		Symbol:         symbol,
		Decimals:       decimals,
		Configured:     true,
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

func (m *EVMMonitor) callTotalSupply(contractAddress string) (*big.Int, error) {
	result, err := m.ethCall(contractAddress, "0x18160ddd")
	if err != nil {
		return big.NewInt(0), err
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
	raw, err := m.rpcRawCall(method, params)
	if err != nil {
		return "", err
	}
	var result string
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if result == "" {
		return "", fmt.Errorf("RPC returned empty result")
	}
	return result, nil
}

func (m *EVMMonitor) rpcRawCall(method string, params []interface{}) (json.RawMessage, error) {
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, err
	}

	resp, err := m.httpClient.Post(m.chain.RPCURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RPC returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed rpcRawResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Error) > 0 && string(parsed.Error) != "null" {
		return nil, fmt.Errorf("RPC error: %s", string(parsed.Error))
	}
	if len(parsed.Result) == 0 || string(parsed.Result) == "null" {
		return nil, fmt.Errorf("RPC returned empty result")
	}
	return parsed.Result, nil
}

func (m *EVMMonitor) findRecentTransfer(change Change) (transferMatch, error) {
	currentBlock, err := m.currentBlockNumber()
	if err != nil {
		return transferMatch{}, err
	}
	fromBlock := uint64(0)
	if currentBlock > defaultLogLookback {
		fromBlock = currentBlock - defaultLogLookback
	}

	walletTopic := addressToTopic(change.WalletAddress)
	var logs []evmLog
	switch change.Direction {
	case "流入":
		logs, err = m.getTransferLogs(change.TokenMint, fromBlock, currentBlock, []interface{}{erc20TransferTopic, nil, walletTopic})
	case "流出":
		logs, err = m.getTransferLogs(change.TokenMint, fromBlock, currentBlock, []interface{}{erc20TransferTopic, walletTopic})
	default:
		inLogs, inErr := m.getTransferLogs(change.TokenMint, fromBlock, currentBlock, []interface{}{erc20TransferTopic, nil, walletTopic})
		outLogs, outErr := m.getTransferLogs(change.TokenMint, fromBlock, currentBlock, []interface{}{erc20TransferTopic, walletTopic})
		if inErr != nil && outErr != nil {
			return transferMatch{}, inErr
		}
		logs = append(inLogs, outLogs...)
	}
	if err != nil {
		return transferMatch{}, err
	}

	var fallback *transferMatch
	for i := len(logs) - 1; i >= 0; i-- {
		match, ok := transferFromLog(logs[i])
		if !ok {
			continue
		}
		if fallback == nil {
			copy := match
			fallback = &copy
		}
		if change.AmountChanged > 0 && match.Amount == change.AmountChanged {
			return match, nil
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return transferMatch{}, fmt.Errorf("no matching transfer log found")
}

func (m *EVMMonitor) currentBlockNumber() (uint64, error) {
	result, err := m.rpcCall("eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}
	value, err := hexToBigInt(result)
	if err != nil {
		return 0, err
	}
	return value.Uint64(), nil
}

func (m *EVMMonitor) getTransferLogs(tokenAddress string, fromBlock, toBlock uint64, topics []interface{}) ([]evmLog, error) {
	filter := map[string]interface{}{
		"address":   tokenAddress,
		"fromBlock": fmt.Sprintf("0x%x", fromBlock),
		"toBlock":   fmt.Sprintf("0x%x", toBlock),
		"topics":    topics,
	}
	raw, err := m.rpcRawCall("eth_getLogs", []interface{}{filter})
	if err != nil {
		return nil, err
	}
	var logs []evmLog
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func transferFromLog(log evmLog) (transferMatch, bool) {
	if len(log.Topics) < 3 {
		return transferMatch{}, false
	}
	amount, err := hexToBigInt(log.Data)
	if err != nil {
		return transferMatch{}, false
	}
	return transferMatch{
		TxHash: log.TransactionHash,
		From:   topicToAddress(log.Topics[1]),
		To:     topicToAddress(log.Topics[2]),
		Amount: bigToUint64(amount),
	}, true
}

func addressToTopic(address string) string {
	clean := strings.TrimPrefix(strings.ToLower(address), "0x")
	return "0x" + strings.Repeat("0", 24) + clean
}

func topicToAddress(topic string) string {
	clean := strings.TrimPrefix(strings.ToLower(topic), "0x")
	if len(clean) < 40 {
		return "0x" + clean
	}
	return "0x" + clean[len(clean)-40:]
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

func holdingPercent(balance, totalSupply *big.Int) float64 {
	if balance == nil || totalSupply == nil || totalSupply.Sign() == 0 {
		return 0
	}
	ratio := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(totalSupply))
	ratio.Mul(ratio, big.NewFloat(100))
	value, _ := ratio.Float64()
	return math.Round(value*10000) / 10000
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
