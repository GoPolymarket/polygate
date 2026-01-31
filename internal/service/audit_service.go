package service

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/GoPolymarket/polygate/internal/model"
)

type AuditService struct {
	logChan chan *model.AuditLog
	logFile *os.File
	buffer  *auditBuffer
	repo    AuditRepo
}

type AuditRepo interface {
	Insert(ctx context.Context, entry *model.AuditLog) error
	List(ctx context.Context, tenantID string, limit int, from, to *time.Time) ([]*model.AuditLog, error)
}

func NewAuditService(logDir string, repo AuditRepo) (*AuditService, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// 简单的按日轮转文件 (MVP)
	filename := filepath.Join(logDir, "audit-"+time.Now().Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	svc := &AuditService{
		logChan: make(chan *model.AuditLog, 1000), // 缓冲区 1000
		logFile: f,
		buffer:  newAuditBuffer(1000),
		repo:    repo,
	}

	// 启动消费者 goroutine
	go svc.processLogs()

	return svc, nil
}

func (s *AuditService) Log(entry *model.AuditLog) {
	if s.buffer != nil {
		s.buffer.Add(entry)
	}
	select {
	case s.logChan <- entry:
		// 写入成功
	default:
		// 缓冲区满，丢弃日志以保护主流程，并打印警告
		// 生产环境应考虑写入备用存储或告警
		log.Println("⚠️ Audit log buffer full, dropping log entry")
	}
}

func (s *AuditService) List(ctx context.Context, tenantID string, limit int, from, to *time.Time) ([]*model.AuditLog, error) {
	if s.repo != nil {
		records, err := s.repo.List(ctx, tenantID, limit, from, to)
		if err == nil {
			return records, nil
		}
	}
	if s.buffer == nil {
		return nil, nil
	}
	return s.buffer.List(tenantID, limit), nil
}

func (s *AuditService) processLogs() {
	encoder := json.NewEncoder(s.logFile)
	for entry := range s.logChan {
		if s.repo != nil {
			if err := s.repo.Insert(context.Background(), entry); err != nil {
				log.Printf("❌ Failed to write audit log to DB: %v", err)
			}
		}
		if err := encoder.Encode(entry); err != nil {
			log.Printf("❌ Failed to write audit log: %v", err)
		}
	}
}

func (s *AuditService) Close() {
	close(s.logChan)
	s.logFile.Close()
}

type auditBuffer struct {
	mu        sync.Mutex
	maxSize   int
	records   []*model.AuditLog
	nextIndex int
}

func newAuditBuffer(maxSize int) *auditBuffer {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &auditBuffer{
		maxSize: maxSize,
		records: make([]*model.AuditLog, 0, maxSize),
	}
}

func (b *auditBuffer) Add(entry *model.AuditLog) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.records) < b.maxSize {
		b.records = append(b.records, entry)
		return
	}
	b.records[b.nextIndex] = entry
	b.nextIndex = (b.nextIndex + 1) % b.maxSize
}

func (b *auditBuffer) List(tenantID string, limit int) []*model.AuditLog {
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit <= 0 || limit > b.maxSize {
		limit = b.maxSize
	}
	results := make([]*model.AuditLog, 0, limit)
	total := len(b.records)
	for i := 0; i < total; i++ {
		idx := (b.nextIndex + total - 1 - i) % total
		entry := b.records[idx]
		if entry == nil {
			continue
		}
		if tenantID != "" && entry.TenantID != tenantID {
			continue
		}
		results = append(results, entry)
		if len(results) >= limit {
			break
		}
	}
	return results
}
