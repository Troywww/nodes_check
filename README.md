# nodes-check

`nodes-check` 是一个面向软路由场景的代理节点筛选与 Cloudflare 发布工具。

核心流程：
- 从订阅中提取 `IP / 端口 / 名称`
- 合并历史成功池
- TCP 预筛
- 两轮 `xray-core` 真延迟测试
- 按分类与推送配置生成最终结果
- 发布到 Cloudflare Worker/KV 与 Cloudflare DNS

## 功能概览

- 内置 `xray-core` 真延迟测试
- WebUI + token 登录保护
- 支持多条订阅链接
- 支持定时自动运行
- 支持历史成功池复用
- 支持 Cloudflare Worker/KV 推送
- 支持 Cloudflare DNS 推送
- 支持 Docker 部署
- 支持自动发布 GHCR 镜像

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
- Cloudflare IP 如果未命中移动、联通、电信，会进入 `官方优选`。
- `其他区域` 表示非 Cloudflare IP，但未被归入香港、亚洲、欧洲、美洲。

## 配置说明

仓库中的 `configs/config.yaml` 是脱敏后的默认配置，可以直接作为模板使用。

部署前至少需要检查这些内容：
- `web.auth_token`
- `probe.template.*`
- `cloudflare.worker.*`
- `cloudflare.dns.*`

不要把真实 token、域名、UUID 等敏感信息提交回仓库。

## 本地运行

Windows PowerShell：

```powershell
./scripts/run-local.ps1
```

Linux / macOS / WSL：

```bash
chmod +x ./scripts/run-local.sh
./scripts/run-local.sh
```

默认访问地址：

```text
http://127.0.0.1:18808
```

## Docker 本地构建

```bash
docker build -t nodes-check .
docker run --rm -p 18808:18808 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/runtime:/app/runtime \
  nodes-check
```

容器首次启动时会自动处理：
- 如果 `/app/configs/config.yaml` 不存在，会自动从镜像内模板复制一份。
- 如果 `/app/configs/subscription_urls.txt` 不存在，也会自动生成模板文件。
- 会自动创建 `/app/runtime/cache`、`/app/runtime/logs`、`/app/runtime/outputs`。

## 飞牛 / NAS 部署

推荐直接使用 GHCR 镜像：

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
```

首次启动后，请检查宿主机目录中是否已经自动生成：

```text
configs/
  config.yaml
  subscription_urls.txt

runtime/
  cache/
  logs/
  outputs/
```

随后按你的实际环境修改 `configs/config.yaml` 即可。

## 镜像说明

当前仓库只保留 Linux 运行所需的 `xray` 资产：
- `bin/xray-linux-64/`

Windows 版 `xray` 与无用压缩包已经移除，不再进入 Linux Docker 镜像。