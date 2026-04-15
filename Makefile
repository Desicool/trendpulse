.PHONY: build test lint vet check simulate run clean

# 构建所有二进制
build:
	go build ./...

# 运行所有测试
test:
	go test ./... -v -count=1

# 运行静态分析
vet:
	go vet ./...

# 代码格式检查
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# 完整检查: build + vet + test
check:
	go build ./...
	go vet ./...
	go test ./... -v -count=1

# 启动 API server
run:
	go run ./cmd/server/main.go

# 运行模拟器 (需要 server 已启动)
simulate:
	go run ./cmd/simulator/main.go

# 清理构建产物
clean:
	rm -rf ./bin ./data
