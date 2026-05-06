package alerts

import (
	"time"
)

type AlertLevel string

const (
	Info     AlertLevel = "INFO"
	Warning  AlertLevel = "WARNING"
	Critical AlertLevel = "CRITICAL"
)

type Alert struct {
	Timestamp     time.Time
	ChainName     string
	ChainType     string
	WalletAddress string
	TokenMint     string
	AlertType     string
	Message       string
	Level         AlertLevel
	Data          map[string]interface{} // Additional data for formatting
}

type Alerter interface {
	SendAlert(alert Alert) error
}

type MultiAlerter struct {
	alerters []Alerter
}

func NewMultiAlerter(alerters ...Alerter) *MultiAlerter {
	return &MultiAlerter{alerters: alerters}
}

func (m *MultiAlerter) SendAlert(alert Alert) error {
	var firstErr error
	for _, alerter := range m.alerters {
		if err := alerter.SendAlert(alert); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
