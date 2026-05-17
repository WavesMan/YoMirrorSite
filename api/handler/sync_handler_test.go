// Sync Handler 测试
// 测试 GetStatus 和 TriggerSync HTTP 端点

package handler

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"yomirrorsite/internal/config"
	"yomirrorsite/internal/syncer"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func makeTestScheduler() *syncer.Scheduler {
	cfg := &config.MirrorConfig{
		Sync: config.SyncConfig{IntervalMinutes: 30, MaxConcurrent: 1},
	}
	return syncer.NewScheduler(nil, cfg)
}

func setupSyncApp(h *SyncHandler) *fiber.App {
	app := fiber.New()
	api := app.Group("/api/sync")
	api.Get("/status", h.GetStatus)
	api.Post("/trigger", h.TriggerSync)
	return app
}

func TestGetStatus_ReturnsOK(t *testing.T) {
	s := makeTestScheduler()
	handler := NewSyncHandler(s)
	app := setupSyncApp(handler)

	req := httptest.NewRequest("GET", "/api/sync/status", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `"success":true`)
	assert.Contains(t, string(body), `"in_progress"`)
}

func TestTriggerSync_EmptyBody(t *testing.T) {
	s := makeTestScheduler()
	handler := NewSyncHandler(s)
	app := setupSyncApp(handler)

	req := httptest.NewRequest("POST", "/api/sync/trigger", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Contains(t, []int{fiber.StatusAccepted, fiber.StatusBadRequest}, resp.StatusCode)
}

func TestTriggerSync_InvalidJSON(t *testing.T) {
	s := makeTestScheduler()
	handler := NewSyncHandler(s)
	app := setupSyncApp(handler)

	req := httptest.NewRequest("POST", "/api/sync/trigger",
		strings.NewReader(`{"software_id":`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Contains(t, []int{fiber.StatusAccepted, fiber.StatusBadRequest}, resp.StatusCode)
}

func TestScheduler_NoSoftware(t *testing.T) {
	cfg := &config.MirrorConfig{
		SoftwareList: []config.SoftwareConfig{},
		Sync:         config.SyncConfig{IntervalMinutes: 30},
	}
	s := syncer.NewScheduler(nil, cfg)
	status := s.GetStatus()
	assert.NotNil(t, status)
	assert.False(t, status.InProgress)

	ctx := context.Background()
	err := s.TriggerSync(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未配置")
}
