package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSoftwareHandler(t *testing.T) {
	h := NewSoftwareHandler(nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.softwareService)
}

func TestNewHandler_Valid(t *testing.T) {
	h := NewSyncHandler(nil)
	assert.NotNil(t, h)
	assert.Nil(t, h.scheduler)
}
