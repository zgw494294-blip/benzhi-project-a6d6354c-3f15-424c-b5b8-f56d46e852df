# BENZHI_README

基于 Go 实现的种子发芽资格评定 Web 项目，一款后端服务，用于管理种子发芽试验、异常复测、规则判定和保藏资格放行。

## 项目说明
- 项目：benzhi-project-a6d6354c-3f15-424c-b5b8-f56d46e852df
- 项目用途：用于支持seed-vigor-gate的核心业务流程。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/seedgate selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-a6d6354c-3f15-424c-b5b8-f56d46e852df-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-a6d6354c-3f15-424c-b5b8-f56d46e852df-arm64 linux/arm64
docker run -it benzhi-project-a6d6354c-3f15-424c-b5b8-f56d46e852df-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/seedgate selfcheck -addr=127.0.0.1:19081`
