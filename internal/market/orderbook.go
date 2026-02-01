package market

import (
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Level represents a single price level in the orderbook
type Level struct {
	Price decimal.Decimal
	Size  decimal.Decimal
}

// Orderbook represents the in-memory state of a market
type Orderbook struct {
	TokenID     string
	Bids        []Level // Sorted High to Low
	Asks        []Level // Sorted Low to High
	LastUpdated time.Time
	mu          sync.RWMutex
}

func NewOrderbook(tokenID string) *Orderbook {
	return &Orderbook{
		TokenID: tokenID,
		Bids:    make([]Level, 0),
		Asks:    make([]Level, 0),
	}
}

// Snapshot replaces the entire book state
func (ob *Orderbook) Snapshot(bids, asks []Level) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Bids = bids
	ob.Asks = asks
	ob.LastUpdated = time.Now()
}

// Update processes a price/size update
// size 0 means remove level
func (ob *Orderbook) Update(side string, priceStr, sizeStr string) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		return err
	}
	size, err := decimal.NewFromString(sizeStr)
	if err != nil {
		return err
	}

	if side == "BUY" {
		ob.updateLevel(&ob.Bids, price, size, true)
	} else {
		ob.updateLevel(&ob.Asks, price, size, false)
	}
	ob.LastUpdated = time.Now()
	return nil
}

func (ob *Orderbook) updateLevel(levels *[]Level, price, size decimal.Decimal, descending bool) {
	// Simple linear scan implementation. 
	// For production HFT with thousands of levels, use a Red-Black Tree or Skip List.
	// For Polymarket (sparse liquidity), slices are cache-friendly and fast enough.
	
	// 1. Find existing level
	idx := -1
	for i, l := range *levels {
		if l.Price.Equal(price) {
			idx = i
			break
		}
	}

	// 2. Delete if size is zero
	if size.IsZero() {
		if idx != -1 {
			// Remove element
			*levels = append((*levels)[:idx], (*levels)[idx+1:]...)
		}
		return
	}

	// 3. Update or Insert
	if idx != -1 {
		(*levels)[idx].Size = size
	} else {
		// Insert
		*levels = append(*levels, Level{Price: price, Size: size})
		// Re-sort
		if descending {
			// Bids: High to Low
			sort.Slice(*levels, func(i, j int) bool {
				return (*levels)[i].Price.GreaterThan((*levels)[j].Price)
			})
		} else {
			// Asks: Low to High
			sort.Slice(*levels, func(i, j int) bool {
				return (*levels)[i].Price.LessThan((*levels)[j].Price)
			})
		}
	}
}

// GetCopy returns a safe copy of the current state (Thread-safe read)
func (ob *Orderbook) GetCopy() (bids, asks []Level) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids = make([]Level, len(ob.Bids))
	copy(bids, ob.Bids)
	asks = make([]Level, len(ob.Asks))
	copy(asks, ob.Asks)
	return
}
