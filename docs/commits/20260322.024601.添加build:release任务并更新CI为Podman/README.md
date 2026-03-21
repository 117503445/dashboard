# 添加 build:release 任务并更新 CI 为 Podman

## 主要内容和目的

参考 `/workspace/project/templates` 项目，添加多平台发布构建任务，并将 GitHub Actions 中的 Docker 替换为 Podman。

## 更改内容描述

1. **`scripts/tasks/build/Taskfile.yml`**
   - 新增 `build:release` 任务，支持 linux/darwin/windows 平台的 amd64/arm64 架构

2. **`scripts/go-scripts/release.go`**
   - 修改构建目标从 `cli` 改为 `dashboard`

3. **`.github/workflows/master.yml`**
   - 将 `docker login` 改为 `podman login`

## 验证方法和结果

```bash
go-task build:release
```

验证结果：
- 成功生成 6 个平台的二进制文件：
  - dashboard-darwin-amd64
  - dashboard-darwin-arm64
  - dashboard-linux-amd64
  - dashboard-linux-arm64
  - dashboard-windows-amd64.exe
  - dashboard-windows-arm64.exe
