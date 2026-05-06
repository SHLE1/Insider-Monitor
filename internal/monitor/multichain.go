package monitor

import (
	"fmt"

	"github.com/accursedgalaxy/insider-monitor/internal/config"
)

type Scanner interface {
	ScanAllWallets() (map[string]*WalletData, error)
	DisplayWalletOverview(walletDataMap map[string]*WalletData)
}

type ChangeEnricher interface {
	EnrichChanges(changes []Change) []Change
}

type MultiChainMonitor struct {
	scanners []Scanner
}

func NewMultiChainMonitor(chains []config.ChainConfig) (*MultiChainMonitor, error) {
	scanners := make([]Scanner, 0, len(chains))
	for _, chain := range chains {
		switch chain.Type {
		case config.ChainTypeSolana:
			scanner, err := NewSolanaMonitor(chain)
			if err != nil {
				return nil, fmt.Errorf("failed to create Solana monitor for %s: %w", chain.Name, err)
			}
			scanners = append(scanners, scanner)
		case config.ChainTypeEVM:
			scanner, err := NewEVMMonitor(chain)
			if err != nil {
				return nil, fmt.Errorf("failed to create EVM monitor for %s: %w", chain.Name, err)
			}
			scanners = append(scanners, scanner)
		default:
			return nil, fmt.Errorf("unsupported chain type %q", chain.Type)
		}
	}
	return &MultiChainMonitor{scanners: scanners}, nil
}

func (m *MultiChainMonitor) ScanAllWallets() (map[string]*WalletData, error) {
	results := make(map[string]*WalletData)
	for _, scanner := range m.scanners {
		data, err := scanner.ScanAllWallets()
		if err != nil {
			return nil, err
		}
		for key, value := range data {
			results[key] = value
		}
	}
	return results, nil
}

func (m *MultiChainMonitor) DisplayWalletOverview(walletDataMap map[string]*WalletData) {
	for _, scanner := range m.scanners {
		scanner.DisplayWalletOverview(walletDataMap)
	}
}

func (m *MultiChainMonitor) EnrichChanges(changes []Change) []Change {
	enriched := changes
	for _, scanner := range m.scanners {
		enricher, ok := scanner.(ChangeEnricher)
		if !ok {
			continue
		}
		enriched = enricher.EnrichChanges(enriched)
	}
	return enriched
}
