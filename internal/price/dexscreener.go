package price

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const dexScreenerTokenURL = "https://api.dexscreener.com/latest/dex/tokens/%s"

type DexScreenerPrice struct {
	data  map[string]PriceData
	mutex sync.RWMutex
}

type dexScreenerResponse struct {
	Pairs []struct {
		BaseToken struct {
			Address string `json:"address"`
		} `json:"baseToken"`
		PriceUSD string `json:"priceUsd"`
	} `json:"pairs"`
}

func NewDexScreenerPrice() *DexScreenerPrice {
	return &DexScreenerPrice{data: make(map[string]PriceData)}
}

func (d *DexScreenerPrice) UpdatePrices(addresses []string) error {
	if len(addresses) == 0 {
		return nil
	}
	for i := 0; i < len(addresses); i += maxTokensPerBatch {
		end := i + maxTokensPerBatch
		if end > len(addresses) {
			end = len(addresses)
		}
		if err := d.updateBatch(addresses[i:end]); err != nil {
			return err
		}
	}
	return nil
}

func (d *DexScreenerPrice) updateBatch(addresses []string) error {
	url := fmt.Sprintf(dexScreenerTokenURL, strings.Join(addresses, ","))
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var parsed dexScreenerResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()
	now := time.Now()
	for _, pair := range parsed.Pairs {
		if pair.BaseToken.Address == "" || pair.PriceUSD == "" {
			continue
		}
		price, err := parsePrice(pair.PriceUSD)
		if err != nil {
			continue
		}
		d.data[strings.ToLower(pair.BaseToken.Address)] = PriceData{
			Price:           price,
			LastUpdated:     now,
			ConfidenceLevel: "medium",
		}
	}
	return nil
}

func (d *DexScreenerPrice) GetPrice(address string) (PriceData, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	data, exists := d.data[strings.ToLower(address)]
	return data, exists
}

func (d *DexScreenerPrice) SetPriceForTest(address string, data PriceData) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.data[strings.ToLower(address)] = data
}
