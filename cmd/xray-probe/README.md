# xray-probe (Go)

最小 Go 版真延迟探测工具。

当前能力：

- `vless + ws + tls`
- `trojan + tls + tcp`
- `trojan + ws + tls`
- 单节点探测
- 文件批量探测
- 重复测试
- JSON 输出

示例：

```bash
go run ./cmd/xray-probe --xray-bin ./bin/xray-linux-64/xray --node 'vless://...'
go run ./cmd/xray-probe --xray-bin ./bin/xray-linux-64/xray --node 'vless://...' --repeat 5 --json
```
