# YoMirrorSite Makefile
# 提供本地开发与 CI/CD 常用命令

.PHONY: test test-unit test-integration test-coverage lint ci clean

# 默认目标
all: test lint

# 全部测试（-short 跳过集成测试）
test: test-unit

# 单元测试（不依赖外部服务）
test-unit:
	go test ./... -short -count=1

# 集成测试（需要 PG + Redis + MinIO）
test-integration:
	@echo "确保本地 PG/Redis/MinIO 已启动（docker compose up -d postgres redis minio）"
	go test ./... -tags=integration -count=1

# 覆盖率报告
test-coverage:
	go test ./... -short -coverprofile=coverage.out -covermode=atomic -count=1
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 代码检查
lint:
	go vet ./...
	@echo "如需 golangci-lint，请自行安装后运行: golangci-lint run ./..."

# CI 完整流程（与 GitHub Actions 一致）
ci: test test-coverage lint

# 构建
build:
	go build -o yomirrorsite ./cmd/api/

# 清理
clean:
	rm -f coverage.out coverage.html unit.out integration.out yomirrorsite
