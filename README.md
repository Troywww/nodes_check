# nodes-check

`nodes-check` 是一个面向软路由场景的代理节点筛选与 Cloudflare 发布工具。

它的核心流程是：
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
- 支持 GitHub Actions 自动发布镜像到 GHCR

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

## 目录结构

```text
cmd/
  server/       Web 服务入口
  xray-probe/   真延迟测试 CLI

internal/
  app/          任务执行器
  classifier/   分类规则
  config/       配置加载
  parser/       订阅解析
  precheck/     TCP 预筛
  probe/        xray 真延迟测试
  publisher/    Cloudflare 发布
  renderer/     结果文件渲染
  selector/     分类选择
  storage/      历史池读写
  subscription/ 订阅抓取
  web/          WebUI

configs/
  config.example.yaml
  subscription_urls.txt

runtime/
  cache/
  logs/
  outputs/

bin/
  xray-linux-64/
  xray-windows-64/
```

## 敏感信息说明

仓库中的示例文件已经脱敏：
- `configs/config.example.yaml`
- `configs/subscription_urls.txt`

建议做法：
- 保留 `configs/config.example.yaml` 作为公开示例
- 本地复制一份自己的 `configs/config.yaml`
- 本地运行时使用 `-config ./configs/config.yaml`
- 不要把真实 token、域名、UUID 再写回示例文件

## 本地运行

### 1. 准备 Go

建议 Go `1.22+`。

### 2. 修改配置

编辑：
- `configs/config.example.yaml`
- `configs/subscription_urls.txt`

至少需要填写：
- `web.auth_token`
- `probe.template.*`
- `cloudflare.worker.*`
- `cloudflare.dns.*`

### 3. 启动 WebUI

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
go run ./cmd/server -config ./configs/config.example.yaml
```

默认访问地址：
- [http://localhost:18808](http://localhost:18808)

## Docker 部署

### 方式一：本地源码构建

适合你自己有源码目录的机器：

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

### 方式二：飞牛 / NAS 直接拉镜像

适合飞牛这类只粘贴 `compose`、不直接拿源码构建的场景。

本仓库已提供 GitHub Actions 自动发布镜像到 GHCR：
- `ghcr.io/troywww/nodes-check:latest`

飞牛可直接使用：

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
    command: ["/app/nodes-check", "-config", "/app/configs/config.example.yaml"]
```

说明：
- 这种方式不需要飞牛现场 `build`。
- 你只需要准备自己的 `configs/config.example.yaml` 或 `configs/config.yaml`。
- 如果 GHCR 包默认是私有的，需要先在 GitHub Packages 里把它改成公开，飞牛才能直接拉取。

## GHCR 自动发布镜像

仓库内置工作流：
- `.github/workflows/publish-image.yml`

触发方式：
- push 到 `main`
- push `v*` 标签
- 手动触发 `workflow_dispatch`

发布后默认会生成这些标签：
- `ghcr.io/troywww/nodes-check:latest`
- `ghcr.io/troywww/nodes-check:sha-...`
- `ghcr.io/troywww/nodes-check:vX.Y.Z`（打 tag 时）

## Docker Hub

如果你更希望用 Docker Hub，也可以手动发布：

```bash
docker build -t troywww/nodes-check:latest .
docker push troywww/nodes-check:latest
```

之后飞牛里的 `compose` 就改成：

```yaml
image: troywww/nodes-check:latest
```

## 说明

- `final_ips.txt` 是最终 KV 内容来源文件
- 历史池保存在 `runtime/cache/history_valid_nodes.txt`
- 若本轮稳定池为空，历史池不会被清空
- 若某个运营商分类也配置进 KV，则会优先写对应域名而不是裸 IP
