package handler

import (
	"testing"

	"yomirrorsite/internal/config"
	"yomirrorsite/internal/syncer"

	"github.com/stretchr/testify/assert"
)

func TestNewSyncHandler(t *testing.T) {
	h := NewSyncHandler(nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.scheduler)
}

func TestSyncHandler_GetStatus_NoSyncer(t *testing.T) {
	// NewScheduler with nil syncer: GetStatus triggers nil deref
	// This is expected — caller must inject a valid syncer
	cfg := &config.MirrorConfig{
		Sync: config.SyncConfig{IntervalMinutes: 30, MaxConcurrent: 1},
	}
	s := syncer.NewScheduler(nil, cfg)
	assert.NotNil(t, s)
	assert.Panics(t, func() {
		s.GetStatus()
	})
}
