# 修正 Docker 镜像名称

## 主要内容和目的

修正 Docker 构建脚本中的镜像名称，使其与项目名称一致。

## 更改内容描述

**scripts/go-scripts/build-docker.go**

- 将镜像名从 `go-template-rpc` 改为 `dashboard`
- 涉及的镜像标签：
  - `117503445/dashboard:latest`
  - `117503445/dashboard:{commit}`
  - `registry.cn-hangzhou.aliyuncs.com/117503445/dashboard:latest`
  - `registry.cn-hangzhou.aliyuncs.com/117503445/dashboard:{commit}`

## 验证方法和结果

- 通过代码审查确认所有镜像名已更新为 `dashboard`
- 与 go.mod 中的模块名 `github.com/117503445/dashboard` 保持一致