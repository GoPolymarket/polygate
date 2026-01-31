package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const ContextAuditLog = "audit_log"

// bodyLogWriter 包装 ResponseWriter 以捕获响应体
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func AuditMiddleware(auditSvc *service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID := uuid.New().String()
		c.Header("X-Request-ID", reqID)

		// 1. 读取请求体 (并写回以便后续 Bind 使用)
		var reqBodyBytes []byte
		if c.Request.Body != nil {
			reqBodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
		}

		// 2. 初始化审计对象并存入 Context
		// 这样业务层 (Service/Handler) 可以往 Context 字段里塞额外信息
		auditEntry := &model.AuditLog{
			ID:        reqID,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			CreatedAt: start,
			Context:   make(map[string]interface{}),
		}
		c.Set(ContextAuditLog, auditEntry)

		// 3. 包装 ResponseWriter 以捕获响应
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		// === 执行业务逻辑 ===
		c.Next()

		// 4. 填充剩余信息 (在请求结束后)
		// 尝试获取 TenantID
		if tenantVal, exists := c.Get(ContextTenantKey); exists {
			auditEntry.TenantID = tenantVal.(*model.Tenant).ID
		}

		auditEntry.RequestBody = redactAuditBody(c.Request.URL.Path, reqBodyBytes)
		auditEntry.StatusCode = c.Writer.Status()
		auditEntry.ResponseBody = redactAuditBody(c.Request.URL.Path, []byte(blw.body.String()))
		auditEntry.LatencyMs = time.Since(start).Milliseconds()

		// 5. 异步发送日志
		auditSvc.Log(auditEntry)
	}
}

// AddAuditContext 辅助函数：允许 Handler/Service 向审计日志添加业务上下文
func AddAuditContext(c *gin.Context, key string, value interface{}) {
	if val, exists := c.Get(ContextAuditLog); exists {
		if entry, ok := val.(*model.AuditLog); ok {
			entry.Context[key] = value
		}
	}
}

func redactAuditBody(path string, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if !isSensitivePath(path) {
		return string(body)
	}
	redacted, ok := redactJSON(body)
	if !ok {
		return "[redacted]"
	}
	return string(redacted)
}

func isSensitivePath(path string) bool {
	switch {
	case strings.HasPrefix(path, "/v1/tenants"):
		return true
	case strings.HasPrefix(path, "/v1/orders"):
		return true
	case strings.HasPrefix(path, "/v1/account"):
		return true
	default:
		return false
	}
}

func redactJSON(body []byte) ([]byte, bool) {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false
	}
	redactValue(&data)
	out, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	return out, true
}

func redactValue(v *interface{}) {
	switch raw := (*v).(type) {
	case map[string]interface{}:
		for key, val := range raw {
			if isSensitiveKey(key) {
				raw[key] = "***"
				continue
			}
			vv := val
			redactValue(&vv)
			raw[key] = vv
		}
	case []interface{}:
		for i, val := range raw {
			vv := val
			redactValue(&vv)
			raw[i] = vv
		}
	}
}

func isSensitiveKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "api_key",
		"api_secret",
		"api_passphrase",
		"l2_api_key",
		"l2_api_secret",
		"l2_api_passphrase",
		"private_key",
		"signature",
		"signer",
		"sig",
		"signature_type",
		"admin_key",
		"admin_secret_key":
		return true
	default:
		return false
	}
}
