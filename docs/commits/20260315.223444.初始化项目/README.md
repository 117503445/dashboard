# 初始化项目

## 主要内容和目的

初始化一个全栈项目模板，包含 Go 后端 RPC 服务和 React 前端应用。

## 更改内容描述

### 后端 (Go)
- `cmd/rpc/` - RPC 服务器入口，支持静态文件服务
- `cmd/cli/` - CLI 工具入口
- `cmd/fc-web/` - 阿里云函数计算 HTTP 触发入口
- `cmd/fc-event/` - 阿里云函数计算事件触发入口
- `pkg/rpc/` - Protobuf 定义和生成的 Connect RPC 代码
- `internal/buildinfo/` - 构建时版本信息

### 前端 (React)
- `fe/` - React 19 + TypeScript + Vite 8 + Tailwind CSS 4
- `fe/src/gen/` - Connect-ES 生成的 RPC 客户端代码

### 构建和部署
- `Taskfile.yml` - Task 任务运行器主配置
- `scripts/tasks/` - 各类任务定义（构建、部署、格式化等）
- `scripts/go-scripts/` - Go 构建脚本
- `scripts/docker/` - Dockerfile 定义
- `scripts/e2e/` - E2E 测试脚本 (Python + Playwright)
- `.github/workflows/master.yml` - GitHub Actions CI/CD 配置

### 其他
- `compose.yaml` - Docker Compose 配置
- `buf.yaml` / `buf.gen.yaml` - Buf Protobuf 工具配置

## 验证方法和结果

1. 前端开发服务器启动正常 (`go-task fe:dev`)
2. RPC 服务器启动正常 (`go-task run:rpc`)
3. E2E 测试通过 (`go-task e2e:run`)