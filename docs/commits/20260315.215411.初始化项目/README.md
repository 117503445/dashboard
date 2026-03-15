# 初始化项目

## 主要内容和目的

基于 go-template 架构模式初始化一个完整的前后端分离项目，包含 Go Connect RPC 后端和 React 前端。

## 更改内容描述

### 后端 (Go)
- 初始化 Go 模块 `github.com/117503445/dashboard`
- 添加 Connect RPC 服务 (`cmd/rpc/`)
- 添加 CLI 工具 (`cmd/cli/`)
- 添加阿里云函数计算支持 (`cmd/fc-web/`, `cmd/fc-event/`)
- 定义 Protobuf API (`pkg/rpc/template.proto`)
- 配置 buf 代码生成 (`buf.yaml`, `buf.gen.yaml`)

### 前端 (React)
- 初始化 Vite + React 19 + TypeScript 项目 (`fe/`)
- 配置 Tailwind CSS 4
- 集成 Connect-ES RPC 客户端
- 添加 shadcn/ui 风格组件

### 构建与开发工具
- 配置 go-task 任务运行器 (`Taskfile.yml`, `scripts/tasks/`)
- 添加构建脚本 (`scripts/go-scripts/`)
- 配置 E2E 测试 (`scripts/e2e/`)
- 添加 Docker 支持 (`scripts/docker/`, `compose.yaml`)

### Taskfile 改进
- `run:rpc-run`: 启动前先杀死旧的 RPC 进程
- `run:rpc-background`: 新增后台启动 RPC 服务任务
- `e2e:run`: 自动依赖 `run:rpc-background`，确保测试前启动服务

## 验证方法和结果

### 后端构建和运行
```bash
go-task build:bin
./data/rpc/rpc
```
RPC 服务成功启动在端口 8080，健康检查接口 `/pkg.rpc.TemplateService/Healthz` 正常响应。

### 前端构建和运行
```bash
go-task fe:dev
```
前端开发服务器成功启动在端口 5173。

### E2E 测试
```bash
go-task e2e:run
```
测试通过：
```
Test Summary:
==================================================
  case1: PASSED
==================================================
```