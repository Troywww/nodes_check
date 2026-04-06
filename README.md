# nodes-check

`nodes-check` 是一个面向软路由场景的代理节点筛选与 Cloudflare 发布工具。

核心流程：
- 从订阅中只提取 `IP / 端口 / 名称`
- 合并历史成功池
- TCP 预筛
- 两轮 `xray-core` 真延迟测试
- 按分类和推送配置生成最终结果
- 推送到 Cloudflare Worker/KV 与 Cloudflare DNS

## 功能概览

- 内置 `xray-core` 真延迟测试
- WebUI + token 登录保护
- 支持多条订阅链接
- 支持定时自动运行
- 支持历史成功池复用
- 支持 Cloudflare Worker/KV 推送
- 支持 Cloudflare DNS 推送
- 支持 Docker 部署
- 支持通过 GitHub 自动发布 GHCR 镜像

## 当前分类规则

大区类：
- 香港
- 亚洲
- 欧洲
- 美洲
- 其他区域

运营商类：
- 移动
- 联通
- 电信
- 官方优选

说明：
- Cloudflare IP 默认不进入普通大区。
- Cloudflare IP 若未命中移动/联通/电信，会进入 `官方优选`。
- `其他区域` 表示非 Cloudflare IP，但未被归入香港/亚洲/欧洲/美洲。

## 配置说明

仓库中的 [config.yaml](D:\project\nodes_check\configs\config.yaml) 是脱敏后的默认配置，可以直接作为模板使用。

部署前至少需要修改这些内容：
- `web.auth_token`
- `probe.template.*`
- `cloudflare.worker.*`
- `cloudflare.dns.*`

不要把真实 token、域名、UUID 再提交回仓库。

## 本地运行

Windows PowerShell：

```powershell
.\scripts\run-local.ps1
```

Linux / WSL：

```bash
sh ./scripts/run-local.sh
```

或直接：

```bash
go run ./cmd/server -config ./configs/config.yaml
```

默认访问地址：
- [http://localhost:18808](http://localhost:18808)

## Docker 部署

### 本地源码构建

```bash
docker build -t nodes-check .
docker run -d \
  --name nodes-check \
  -p 18808:18808 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/runtime:/app/runtime \
  nodes-check
```

或：

```bash
docker compose up -d --build
```

说明：
- 容器默认读取 `/app/configs/config.yaml`
- 如果挂载了 `./configs:/app/configs`，宿主机目录里必须有 `config.yaml`

### 飞牛 / NAS 直接拉 GHCR 镜像

```yaml
services:
  nodes-check:
    image: ghcr.io/troywww/nodes-check:latest
    container_name: nodes-check
    ports:
      - "18808:18808"
    volumes:
      - ./configs:/app/configs
      - ./runtime:/app/runtime
    restart: unless-stopped
    command: ["/app/nodes-check", "-config", "/app/configs/config.yaml"]
```

说明：
- 这种方式不需要 NAS 现场 `build`
- 你只需要准备自己的 `configs` 和 `runtime` 目录
- 如果 GHCR 包默认是私有的，需要先在 GitHub Packages 里把它改成公开

## 镜像说明

- Linux 镜像只包含 Linux 版 `xray`
- Windows 版 `xray.exe` 不会进入 Docker Linux 镜像
- 镜像内只保留运行必需文件
- `runtime` 目录在镜像中只创建空目录，实际运行数据建议通过卷挂载保存

## 说明

- `final_ips.txt` 是最终 KV 内容来源文件
- 历史池保存在 `runtime/cache/history_valid_nodes.txt`
- 若本轮稳定池为空，历史池不会被清空
- 若某个运营商分类也配置进 KV，则会优先写对应域名而不是裸 IP