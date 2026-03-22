# Agent 列表按 port 排序

## 主要内容和目的

让后端 API 返回的 Agent 列表按照 `HubPort` 升序排列，使列表展示更有序。

## 更改内容描述

- 在 `cmd/dashboard/handler.go` 中添加 `sort` 包导入
- `ListAgents` 函数：在返回真实 Agent 数据前按 `HubPort` 排序
- `listMockAgents` 函数：在返回模拟 Agent 数据前同样按 `HubPort` 排序

```go
sort.Slice(agents, func(i, j int) bool {
    return agents[i].HubPort < agents[j].HubPort
})
```

## 验证方法和结果

- 编译通过：`go build ./cmd/dashboard`