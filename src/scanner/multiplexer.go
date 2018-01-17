package scanner

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

// Multiplexer manager of scanner
type Multiplexer struct {
	scannerMap   map[string]Scanner
	outChan      chan DepositNote
	scannerCount int
	quit         chan struct{}
	done         chan struct{}
	log          logrus.FieldLogger
	sync.RWMutex
}

// NewMultiplexer create multiplexer instance
func NewMultiplexer(log logrus.FieldLogger) *Multiplexer {
	return &Multiplexer{
		scannerMap:   map[string]Scanner{},
		outChan:      make(chan DepositNote, 1000),
		scannerCount: 0,
		log:          log.WithField("prefix", "scanner.multiplex"),
		quit:         make(chan struct{}),
		done:         make(chan struct{}, 1),
	}
}

// AddScanner add scanner of coinType
func (m *Multiplexer) AddScanner(scanner Scanner, coinType string) error {
	if scanner == nil {
		return errors.New("nil scanner")
	}
	m.RWMutex.Lock()
	defer m.RWMutex.Unlock()
	_, existsScanner := m.scannerMap[coinType]
	if existsScanner {
		return fmt.Errorf("scanner of coinType %s already exists", coinType)
	}

	m.scannerMap[coinType] = scanner
	m.scannerCount++
	return nil
}

// AddScanAddress adds new scan address to scanner according to coinType
func (m *Multiplexer) AddScanAddress(depositAddr, coinType string) error {
	m.RWMutex.Lock()
	defer m.RWMutex.Unlock()
	scanner, existsScanner := m.scannerMap[coinType]
	if !existsScanner {
		return errors.New("unknown cointype")
	}
	return scanner.AddScanAddress(depositAddr, coinType)
}

// Multiplex forward multi-scanner deposit to a shared aggregate channel, think of "Goroutine merging channel"
func (m *Multiplexer) Multiplex() error {
	log := m.log.WithField("scanner-count", m.scannerCount)
	log.Info("Start multiplex service")
	defer func() {
		log.Info("Multiplex service closed")
		close(m.done)
	}()
	var wg sync.WaitGroup
	for _, scan := range m.scannerMap {
		wg.Add(1)
		go func(scan Scanner) {
			defer log.Info("Scan goroutine exited")
			defer wg.Done()
			for dv := range scan.GetDeposit() {
				m.outChan <- dv
			}
		}(scan)
	}
	wg.Wait()

	return nil
}

// GetDeposit returns deposit values channel.
func (m *Multiplexer) GetDeposit() <-chan DepositNote {
	return m.outChan
}

// GetScannerCount returns scanner count.
func (m *Multiplexer) GetScannerCount() int {
	return m.scannerCount
}

// Shutdown shutdown the multiplexer
func (m *Multiplexer) Shutdown() {
	m.log.Info("Closing Multiplexer")
	close(m.quit)
	close(m.outChan)
	m.log.Info("Waiting for Multiplexer to stop")
	<-m.done
}

// GetScanner returns Scanner according to coinType
func (m *Multiplexer) GetScanner(coinType string) Scanner {
	scanner, existsScanner := m.scannerMap[coinType]
	if !existsScanner {
		return nil
	}
	return scanner
}
