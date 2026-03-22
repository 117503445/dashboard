# CI 调整执行顺序：先 release 后构建镜像

## 主要内容和目的

调整 GitHub Actions CI 流程执行顺序，将构建 Docker 镜像移至 GitHub Release 之后执行。

## 更改内容描述

修改 `.github/workflows/master.yml`：

**调整前顺序：**
1. 构建并推送 Docker 镜像
2. 构建 release
3. 创建 GitHub release 并上传资源

**调整后顺序：**
1. 构建 release
2. 创建 GitHub release 并上传资源
3. 构建并推送 Docker 镜像

## 验证方法和结果

- 本地已确认 YAML 语法正确