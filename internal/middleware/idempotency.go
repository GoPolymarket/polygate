package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/gin-gonic/gin"
)

const HeaderIdempotencyKey = "X-Idempotency-Key"

type IdempotencyRecord struct {
	Status     int
	Body       []byte
	CreatedAt  time.Time
	Processing bool // 正在处理中，用于防止并发竞争
}

type IdempotencyStore interface {
	// GetOrLock returns (record, true) if exists; (nil,false) if newly locked by caller.
	GetOrLock(key string) (*IdempotencyRecord, bool)
	Save(key string, status int, body []byte)
	Unlock(key string)
}

// InMemIdempotencyStore 用于 MVP 演示，生产环境请用 Redis
type InMemIdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]*IdempotencyRecord // Key: TenantID + ":" + IdempotencyKey
}

func NewInMemIdempotencyStore() *InMemIdempotencyStore {
	return &InMemIdempotencyStore{
		records: make(map[string]*IdempotencyRecord),
	}
}

// GetOrLock 尝试获取记录。如果不存在，则锁定并返回 nil（表示你是第一个）。
// 如果正在处理，返回 Processing=true。如果已完成，返回完整记录。
func (s *InMemIdempotencyStore) GetOrLock(key string) (*IdempotencyRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rec, ok := s.records[key]; ok {
		return rec, true // 命中缓存或正在处理
	}

	// 锁定该 Key
	s.records[key] = &IdempotencyRecord{
		Processing: true,
		CreatedAt:  time.Now(),
	}
	return nil, false // 未命中，你获得了锁
}

func (s *InMemIdempotencyStore) Save(key string, status int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[key] = &IdempotencyRecord{
		Status:     status,
		Body:       body,
		CreatedAt:  time.Now(),
		Processing: false,
	}
}

func (s *InMemIdempotencyStore) Unlock(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, key)
}

// IdempotencyMiddleware 幂等性中间件
func IdempotencyMiddleware(store IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 检查 Header
		idemKey := c.GetHeader(HeaderIdempotencyKey)
		if idemKey == "" {
			c.Next()
			return
		}

		// 2. 获取租户 (确保在 Auth 之后)
		tenantVal, exists := c.Get(ContextTenantKey)
		if !exists {
			c.Next() // 理论上不会发生
			return
		}
		tenant := tenantVal.(*model.Tenant)

		fullKey := tenant.ID + ":" + idemKey

		// 3. 检查存储
		record, hit := store.GetOrLock(fullKey)
		if hit {
			if record.Processing {
				// 正在处理中（并发请求）：返回 429 或 409
				c.JSON(http.StatusConflict, gin.H{"error": "request in progress"})
				c.Abort()
				return
			}
			// 已处理完成：直接返回缓存的响应
			// 注意：这里需要设置正确的 Content-Type
			c.Data(record.Status, "application/json; charset=utf-8", record.Body)
			c.Abort()
			return
		}

		// 4. 捕获响应
		// 我们使用 Gin 的 ResponseWriter 钩子来捕获输出
		w := &responseBodyWriter{body: nil, ResponseWriter: c.Writer}
		c.Writer = w

		c.Next()

		// 5. 保存结果 (只有成功或特定的业务失败才保存？通常全保存)
		// 如果 Handler 处理中 panic 了，这里可能不会执行（除非有 Recover 中间件在更外层）
		// 简单起见，我们保存所有非 500 的结果，或者是全部保存。
		if c.Writer.Status() < 500 {
			store.Save(fullKey, c.Writer.Status(), w.body)
		} else {
			// 如果是服务器内部错误，通常允许重试，所以解锁但不保存结果
			store.Unlock(fullKey)
		}
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body []byte
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	// 捕获 Body
	// 注意：对于大响应这可能消耗内存，但订单响应通常很小
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}
