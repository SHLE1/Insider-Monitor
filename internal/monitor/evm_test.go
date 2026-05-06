package monitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/accursedgalaxy/insider-monitor/internal/config"
	"github.com/accursedgalaxy/insider-monitor/internal/price"
)

func TestEVMMonitorScansNativeAndConfiguredToken(t *testing.T) {
	wallet := "0x1111111111111111111111111111111111111111"
	token := "0x2222222222222222222222222222222222222222"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		result := "0x0"
		switch req.Method {
		case "eth_chainId":
			result = "0x38"
		case "eth_getBalance":
			result = "0xde0b6b3a7640000"
		case "eth_call":
			call := req.Params[0].(map[string]interface{})
			data := call["data"].(string)
			switch {
			case strings.HasPrefix(data, "0x70a08231"):
				result = "0xde0b6b3a7640000"
			case data == "0x313ce567":
				result = "0x12"
			case data == "0x95d89b41":
				result = "0x" +
					"0000000000000000000000000000000000000000000000000000000000000020" +
					"0000000000000000000000000000000000000000000000000000000000000004" +
					"5553445400000000000000000000000000000000000000000000000000000000"
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		})
	}))
	defer server.Close()

	mon, err := NewEVMMonitor(config.ChainConfig{
		Type:         config.ChainTypeEVM,
		Name:         "BSC",
		RPCURL:       server.URL,
		ChainID:      56,
		NativeSymbol: "BNB",
		Wallets:      []string{wallet},
		Scan: config.ScanConfig{
			Tokens: []config.TokenConfig{{Address: token}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := mon.ScanAllWallets()
	if err != nil {
		t.Fatal(err)
	}

	walletData := data[WalletDataKey("BSC", wallet)]
	if walletData == nil {
		t.Fatal("expected wallet data")
	}
	if walletData.TokenAccounts[nativeAssetAddress].Symbol != "BNB" {
		t.Fatalf("expected BNB native balance, got %#v", walletData.TokenAccounts[nativeAssetAddress])
	}
	tokenInfo := walletData.TokenAccounts[strings.ToLower(token)]
	if tokenInfo.Symbol != "USDT" || tokenInfo.Decimals != 18 {
		t.Fatalf("expected USDT token metadata, got %#v", tokenInfo)
	}
}

func TestEVMEnrichPricesWritesUSDFields(t *testing.T) {
	mon := &EVMMonitor{
		chain:        config.ChainConfig{Name: "BSC"},
		priceService: price.NewDexScreenerPrice(),
	}
	mon.priceService.SetPriceForTest("0x2222222222222222222222222222222222222222", price.PriceData{
		Price:           2,
		ConfidenceLevel: "medium",
	})

	data := map[string]*WalletData{
		"BSC:wallet1": {
			ChainName:     "BSC",
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{
				"0x2222222222222222222222222222222222222222": {
					Balance:  1_000_000,
					Symbol:   "TKN",
					Decimals: 6,
				},
			},
		},
	}

	if err := mon.enrichPrices(data); err != nil {
		t.Fatal(err)
	}

	info := data["BSC:wallet1"].TokenAccounts["0x2222222222222222222222222222222222222222"]
	if info.USDPrice != 2 || info.USDValue != 2 || info.ConfidenceLevel != "medium" {
		t.Fatalf("expected USD fields to be persisted, got %#v", info)
	}
}

func TestDetectChangesIncludesChain(t *testing.T) {
	oldData := map[string]*WalletData{
		"BSC:wallet1": {
			ChainName:     "BSC",
			ChainType:     config.ChainTypeEVM,
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{
				"native": {Balance: 100, Symbol: "BNB", Decimals: 18},
			},
		},
	}
	newData := map[string]*WalletData{
		"BSC:wallet1": {
			ChainName:     "BSC",
			ChainType:     config.ChainTypeEVM,
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{
				"native": {Balance: 200, Symbol: "BNB", Decimals: 18},
			},
		},
	}

	changes := DetectChanges(oldData, newData, 20)
	if len(changes) != 1 {
		t.Fatalf("expected one change, got %d", len(changes))
	}
	if changes[0].ChainName != "BSC" || changes[0].ChainType != config.ChainTypeEVM {
		t.Fatalf("expected chain metadata, got %#v", changes[0])
	}
}

func TestDetectChangesSkipsConfiguredTokenAddedToBaseline(t *testing.T) {
	oldData := map[string]*WalletData{
		"BSC:wallet1": {
			ChainName:     "BSC",
			ChainType:     config.ChainTypeEVM,
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{},
		},
	}
	newData := map[string]*WalletData{
		"BSC:wallet1": {
			ChainName:     "BSC",
			ChainType:     config.ChainTypeEVM,
			WalletAddress: "wallet1",
			TokenAccounts: map[string]TokenAccountInfo{
				"0x2222222222222222222222222222222222222222": {
					Balance:    100,
					Symbol:     "TKN",
					Decimals:   18,
					Configured: true,
				},
			},
		},
	}

	changes := DetectChanges(oldData, newData, 20)
	if len(changes) != 0 {
		t.Fatalf("expected configured token addition to be treated as baseline, got %d changes", len(changes))
	}
}
