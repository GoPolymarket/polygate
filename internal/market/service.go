package market

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/GoPolymarket/polygate/internal/pkg/logger"
	"github.com/gorilla/websocket"
)

const (
	WSURL           = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	ReconnBaseDelay = 1 * time.Second
	ReconnMaxDelay  = 30 * time.Second
	PingPeriod      = 15 * time.Second // Keep-alive interval
)

type MarketService struct {
	conn        *websocket.Conn
	mu          sync.RWMutex
	books       map[string]*Orderbook
	subs        []string // List of TokenIDs we want to subscribe to
	ctx         context.Context
	cancel      context.CancelFunc
	isConnected bool
}

func NewMarketService() *MarketService {
	ctx, cancel := context.WithCancel(context.Background())
	return &MarketService{
		books: make(map[string]*Orderbook),
		subs:  make([]string, 0),
		ctx:   ctx,
		cancel: cancel,
	}
}

// Start launches the connection loop in a background goroutine
func (s *MarketService) Start() {
	go s.runLoop()
}

// Stop closes the service
func (s *MarketService) Stop() {
	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}
}

// Subscribe adds tokenIDs to the subscription list and updates the connection if active
func (s *MarketService) Subscribe(tokenIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add unique IDs
	updates := false
	for _, id := range tokenIDs {
		found := false
		for _, existing := range s.subs {
			if existing == id {
				found = true
				break
			}
		}
		if !found {
			s.subs = append(s.subs, id)
			// Initialize empty book
			s.books[id] = NewOrderbook(id)
			updates = true
		}
	}

	if updates && s.isConnected {
		// Send subscription message
		s.sendSubscribe(tokenIDs)
	}
}

func (s *MarketService) GetBook(tokenID string) *Orderbook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.books[tokenID]
}

func (s *MarketService) runLoop() {
	delay := ReconnBaseDelay

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if err := s.connect(); err != nil {
			logger.Error("Connection failed", "error", err, "retry_in", delay)
			time.Sleep(delay)
			delay *= 2
			if delay > ReconnMaxDelay {
				delay = ReconnMaxDelay
			}
			continue
		}

		// Connected successfully
		delay = ReconnBaseDelay
		s.mu.Lock()
		s.isConnected = true
		s.mu.Unlock()

		// Resubscribe to all
		s.mu.RLock()
		allSubs := s.subs
		s.mu.RUnlock()
		if len(allSubs) > 0 {
			if err := s.sendSubscribe(allSubs); err != nil {
				logger.Error("Failed to resubscribe", "error", err)
				s.conn.Close()
				continue
			}
		}

		// Read Loop
		s.readLoop()
		
		s.mu.Lock()
		s.isConnected = false
		s.mu.Unlock()
	}
}

func (s *MarketService) connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(WSURL, nil)
	if err != nil {
		return err
	}
	s.conn = conn
	
	// Zombie Check: Set ReadDeadline
	// If we don't receive ANY data (or Pong) within PingPeriod + Buffer, we assume dead.
	readTimeout := PingPeriod + 10*time.Second
	s.conn.SetReadDeadline(time.Now().Add(readTimeout))
	
	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})
	
	// Start Pinger
	go func() {
		ticker := time.NewTicker(PingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.isConnected || s.conn == nil {
					s.mu.Unlock()
					return
				}
				err := s.conn.WriteMessage(websocket.PingMessage, []byte{})
				s.mu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	return nil
}

type WSMessage struct {
	EventType string          `json:"event_type"` // "book" or "price_change"
	Market    string          `json:"market"`     // TokenID (asset_id)
	Bids      []PriceLevelRaw `json:"bids"`
	Asks      []PriceLevelRaw `json:"asks"`
	Hash      string          `json:"hash"` // If present, it's a snapshot
}

type PriceLevelRaw struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

func (s *MarketService) readLoop() {
	defer s.conn.Close()
	
	readTimeout := PingPeriod + 10*time.Second

	for {
		s.conn.SetReadDeadline(time.Now().Add(readTimeout))
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			logger.Error("Read error", "error", err)
			return
		}

		var msg []WSMessage
		// Polymarket sends array of messages
		if err := json.Unmarshal(message, &msg); err != nil {
			// Try single object just in case
			var single WSMessage
			if err2 := json.Unmarshal(message, &single); err2 == nil {
				msg = []WSMessage{single}
			} else {
				// Keep alive or control message?
				continue
			}
		}

		for _, m := range msg {
			if m.EventType == "book" && m.Market != "" {
				s.processBookMessage(m)
			}
		}
	}
}

func (s *MarketService) processBookMessage(msg WSMessage) {
	s.mu.RLock()
	book, exists := s.books[msg.Market]
	s.mu.RUnlock()

	if !exists {
		return
	}

	for _, b := range msg.Bids {
		if b.Size == "0" {
			// Fast path for deletion
			book.Update("BUY", b.Price, "0")
		} else {
			book.Update("BUY", b.Price, b.Size)
		}
	}
	for _, a := range msg.Asks {
		if a.Size == "0" {
			book.Update("SELL", a.Price, "0")
		} else {
			book.Update("SELL", a.Price, a.Size)
		}
	}
}

func (s *MarketService) sendSubscribe(tokenIDs []string) error {
	msg := map[string]interface{}{
		"type":       "subscribe",
		"assets_ids": tokenIDs,
		"channel_name": "book",
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return fmt.Errorf("no connection")
	}
	return s.conn.WriteJSON(msg)
}