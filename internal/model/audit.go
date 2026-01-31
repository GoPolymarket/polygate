package model

import (
	"time"
)

// AuditLog 代表一次完整的操作审计记录
type AuditLog struct {
	ID            string    `json:"id"`             // 唯一请求 ID (UUID)
	TenantID      string    `json:"tenant_id"`      // 租户 ID
	Method        string    `json:"method"`         // HTTP 方法
	Path          string    `json:"path"`           // 请求路径
	IP            string    `json:"ip"`             // 客户端 IP
	UserAgent     string    `json:"user_agent"`     // 客户端 UA
	
	// 请求详情
	RequestBody   string    `json:"request_body"`   // 请求体 (脱敏后)
	RequestHeader string    `json:"request_header"` // 关键 Header
	
	// 响应详情
	StatusCode    int       `json:"status_code"`    // HTTP 状态码
	ResponseBody  string    `json:"response_body"`  // 响应体
	LatencyMs     int64     `json:"latency_ms"`     // 耗时 (毫秒)
	
	// 业务上下文 (JSON string)
	// 这里可以存储 SDK 调用参数、生成的签名、上游返回的原始错误等
	Context       map[string]interface{} `json:"context"` 

	CreatedAt     time.Time `json:"created_at"`
}
