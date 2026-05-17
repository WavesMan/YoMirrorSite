// 软件处理器 HTTP 层测试
// 使用 Fiber test 工具验证参数解析和状态码

package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func setupSoftwareApp(h *SoftwareHandler) *fiber.App {
	app := fiber.New()
	api := app.Group("/api/mirror")
	api.Get("/stats", h.GetStats)
	return app
}

func TestGetStats_NilService_Panics(t *testing.T) {
	assert.Panics(t, func() {
		h := &SoftwareHandler{softwareService: nil}
		app := setupSoftwareApp(h)
		req := httptest.NewRequest("GET", "/api/mirror/stats", nil)
		app.Test(req)
	})
}

// 辅助：读取 JSON 响应
func readJSONBody(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}
