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
- Cloudflare IP 默认不进入普通大区
- Cloudflare IP 若未命中移动/联通/电信，会进入 `官方优选`
- `其他区域` 表示非 Cloudflare IP，但未被归入香港/亚洲/欧洲/美洲

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

## 发布前的敏感信息说明

仓库中的示例文件已经脱敏：
- `configs/config.example.yaml`
- `configs/subscription_urls.txt`

上传到 GitHub 前，请确认你本地实际运行时使用的是自己的私有配置，不要把真实值写回示例文件。

建议做法：
- 保留 `configs/config.example.yaml` 作为公开示例
- 本地复制一份自己的 `configs/config.yaml`
- 本地运行时用 `-config ./configs/config.yaml`

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

## Docker 运行

### 方式一：docker compose

```bash
docker compose up -d --build
```

默认映射：
- `18808:18808`

配置和运行数据通过卷挂载：
- `./configs -> /app/configs`
- `./runtime -> /app/runtime`

### 方式二：直接 docker build / run

```bash
docker build -t nodes-check .
docker run -d \
  --name nodes-check \
  -p 18808:18808 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/runtime:/app/runtime \
  nodes-check
```

## 一键启动说明

仓库已提供：
- `scripts/run-local.sh`
- `scripts/run-local.ps1`

它们会：
- 编译 `cmd/server`
- 按配置启动 Web 服务

## 上传 GitHub 前还需要做的事

当前目录还不是 git 仓库。如果你要上传 GitHub，可以在本地执行：

```bash
git init
git add .
git commit -m "Initial commit"
```

然后推送到你的 GitHub 仓库。

## 说明

- `final_ips.txt` 是最终 KV 内容来源文件
- 历史池保存在 `runtime/cache/history_valid_nodes.txt`
- 若本轮稳定池为空，历史池不会被清空
- 若某个运营商分类也配置进 KV，则会优先写对应域名而不是裸 IP
