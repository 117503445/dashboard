# 简化开发环境启动命令

## 主要内容和目的

将多个分散的 task 命令整合为一个 `task run:dev` 命令，实现一键启动完整开发环境。

## 更改内容描述

### Taskfile.yml 重构
- 新增 `sshole-build` task：从 GitHub 克隆 sshole 仓库并编译到 `./data/bin`（锁定 commit，存在二进制则跳过）
- 重构 `dev` task：整合前端编译、后端编译、sshole 编译、服务启动
- 删除冗余的 `dashboard-run`、`dashboard-background` 等 task
- 日志输出到 `./data/runs/{时间戳}/` 目录下的 3 个 log 文件

### Docker 配置简化
- 简化 `compose.yaml` 中的 BASE_IMAGE 配置
- 简化 `dev.Dockerfile`

### 其他
- 将 `compose.override.yaml` 加入 `.gitignore`（包含敏感信息）

## 验证方法和结果

```bash
task run:dev
```

验证结果：
- Dashboard 前端访问 http://localhost:8080/ ✅
- Agent 列表显示 `test-agent` (hubPort: 10000, online: true) ✅
- 日志文件正确生成在 `./data/runs/` 目录 ✅