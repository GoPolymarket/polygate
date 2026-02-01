package market

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/GoPolymarket/polygate/internal/pkg/logger"
	"github.com/gorilla/websocket"
)

type UserStream struct {
	conn      *websocket.Conn
	apiKey    string
	apiSecret string
	passphrase string
	fills     []Fill
	mu        sync.RWMutex
}

type Fill struct {
	Market    string    `json:"market"`
	Price     string    `json:"price"`
	Size      string    `json:"size"`
	Side      string    `json:"side"`
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"fill_id"`
}

func NewUserStream(key, secret, passphrase string) *UserStream {
	return &UserStream{
		apiKey:     key,
		apiSecret:  secret,
		passphrase: passphrase,
		fills:      make([]Fill, 0),
	}
}

func (s *UserStream) Start() {
	go s.connectAndRead()
}

func (s *UserStream) GetFills() []Fill {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return copy
	res := make([]Fill, len(s.fills))
	copy(res, s.fills)
	return res
}

func (s *UserStream) connectAndRead() {
	// 1. Dial
	conn, _, err := websocket.DefaultDialer.Dial(WSURL, nil)
	if err != nil {
		logger.Error("Dial failed", "error", err)
		return
	}
	s.conn = conn
	defer conn.Close()

	// 2. Auth
	if err := s.authenticate(); err != nil {
		logger.Error("Auth failed", "error", err)
		return
	}

	// 3. Subscribe
	subMsg := map[string]interface{}{
		"type":         "subscribe",
		"channel_name": "user",
	}
	if err := conn.WriteJSON(subMsg); err != nil {
		logger.Error("Subscribe failed", "error", err)
		return
	}

	// 4. Read Loop
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logger.Error("Read failed", "error", err)
			return
		}
		s.handleMessage(msg)
	}
}

func (s *UserStream) authenticate() error {
	// Timestamp
	ts := fmt.Sprintf("%d", time.Now().Unix())
	signStr := ts + "GET" + "/ws/market"
	
	// HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(s.apiSecret))
	mac.Write([]byte(signStr))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	authMsg := map[string]string{
		"type":        "auth",
		"key":         s.apiKey,
		"signature":   sig,
		"timestamp":   ts,
		"passphrase":  s.passphrase,
	}
	
	return s.conn.WriteJSON(authMsg)
}

func (s *UserStream) handleMessage(raw []byte) {
	// Parse generic to check event type
	var msgs []WSMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		var single WSMessage
		if err2 := json.Unmarshal(raw, &single); err2 == nil {
			msgs = []WSMessage{single}
		}
	}

	for _, m := range msgs {
		if m.EventType == "fills" {
			// Parse Fills
			// Note: The structure of 'fills' event might differ from 'book'
			// For MVP, we just log it
			logger.Info("Fill received", "market", m.Market)
			
			// In real impl, parse m.Data or m.Fills list and append to s.fills
			// s.mu.Lock()
			// s.fills = append(s.fills, ...)
			// s.mu.Unlock()
		}
	}
}
