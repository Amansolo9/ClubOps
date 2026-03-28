package services

import (
	"strings"
	"time"

	"clubops_portal/fullstack/internal/store"
)

type AuditService struct {
	store *store.SQLiteStore
}

func NewAuditService(st *store.SQLiteStore) *AuditService { return &AuditService{store: st} }

func (s *AuditService) ParseEntity(path string) (string, string) {
	p := strings.Trim(path, "/")
	parts := strings.Split(p, "/")
	if len(parts) == 0 {
		return "", ""
	}
	if parts[0] == "api" && len(parts) > 1 {
		parts = parts[1:]
	}
	entity := parts[0]
	id := ""
	if len(parts) > 1 {
		id = parts[1]
	}
	return entity, id
}

func (s *AuditService) Write(userID *int64, method, path string, before any, after any) {
	entity, id := s.ParseEntity(path)
	_ = s.store.InsertAuditLog(userID, method, path, entity, id, before, after)
}

func (s *AuditService) StartRetentionWorker(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		_ = s.store.CleanupAuditLogs()
	}
}
