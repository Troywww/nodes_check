# nodes-check

`nodes-check` 鏄竴涓潰鍚戣蒋璺敱鍦烘櫙鐨勪唬鐞嗚妭鐐圭瓫閫変笌 Cloudflare 鍙戝竷宸ュ叿銆?
瀹冪殑鏍稿績娴佺▼鏄細
- 浠庤闃呬腑鍙彁鍙?`IP / 绔彛 / 鍚嶇О`
- 鍚堝苟鍘嗗彶鎴愬姛姹?- TCP 棰勭瓫
- 涓よ疆 `xray-core` 鐪熷欢杩熸祴璇?- 鎸夊垎绫诲拰鎺ㄩ€侀厤缃敓鎴愭渶缁堢粨鏋?- 鎺ㄩ€佸埌 Cloudflare Worker/KV 涓?Cloudflare DNS

## 鍔熻兘姒傝

- 鍐呯疆 `xray-core` 鐪熷欢杩熸祴璇?- WebUI + token 鐧诲綍淇濇姢
- 鏀寔澶氭潯璁㈤槄閾炬帴
- 鏀寔瀹氭椂鑷姩杩愯
- 鏀寔鍘嗗彶鎴愬姛姹犲鐢?- 鏀寔 Cloudflare Worker/KV 鎺ㄩ€?- 鏀寔 Cloudflare DNS 鎺ㄩ€?- 鏀寔 Docker 閮ㄧ讲
- 鏀寔 GitHub Actions 鑷姩鍙戝竷闀滃儚鍒?GHCR / Docker Hub

## 褰撳墠鍒嗙被瑙勫垯

澶у尯绫伙細
- 棣欐腐
- 浜氭床
- 娆ф床
- 缇庢床
- 鍏朵粬鍖哄煙

杩愯惀鍟嗙被锛?- 绉诲姩
- 鑱旈€?- 鐢典俊
- 瀹樻柟浼橀€?
璇存槑锛?- Cloudflare IP 榛樿涓嶈繘鍏ユ櫘閫氬ぇ鍖恒€?- Cloudflare IP 鑻ユ湭鍛戒腑绉诲姩/鑱旈€?鐢典俊锛屼細杩涘叆 `瀹樻柟浼橀€塦銆?- `鍏朵粬鍖哄煙` 琛ㄧず闈?Cloudflare IP锛屼絾鏈褰掑叆棣欐腐/浜氭床/娆ф床/缇庢床銆?
## 鏁忔劅淇℃伅璇存槑

浠撳簱涓殑 `configs/config.yaml` 鏄劚鏁忓悗鐨勬寮忛粯璁ら厤缃紝鍙互鐩存帴琚媺鍙栧拰鎸傝浇浣跨敤銆?
寤鸿鍋氭硶锛?- 淇濈暀浠撳簱閲岀殑 `configs/config.yaml` 浣滀负榛樿妯℃澘
- 鍦ㄩ儴缃插墠鎶婂叾涓殑鐪熷疄 token銆佸煙鍚嶃€乁UID 鏀规垚浣犺嚜宸辩殑鍊?- 濡傞渶淇濈暀绉佹湁鍓湰锛屽彲鍙﹀缓 `configs/*.local.yaml`
- 涓嶈鎶婄湡瀹炴晱鎰熷€兼彁浜ゅ洖浠撳簱

## 鏈湴杩愯

### 1. 鍑嗗 Go

寤鸿 Go `1.22+`銆?
### 2. 淇敼閰嶇疆

鑷冲皯闇€瑕佸～鍐欙細
- `web.auth_token`
- `probe.template.*`
- `cloudflare.worker.*`
- `cloudflare.dns.*`

### 3. 鍚姩 WebUI

Windows PowerShell锛?
```powershell
.\scripts\run-local.ps1
```

Linux / WSL锛?
```bash
sh ./scripts/run-local.sh
```

鎴栫洿鎺ワ細

```bash
go run ./cmd/server -config ./configs/config.yaml
```

榛樿璁块棶鍦板潃锛?- [http://localhost:18808](http://localhost:18808)

## Docker 閮ㄧ讲

### 鏂瑰紡涓€锛氭湰鍦版簮鐮佹瀯寤?
閫傚悎浣犺嚜宸辨湁婧愮爜鐩綍鐨勬満鍣細

```bash
docker build -t nodes-check .
docker run -d \
  --name nodes-check \
  -p 18808:18808 \
  -v $(pwd)/configs:/app/configs \
  -v $(pwd)/runtime:/app/runtime \
  nodes-check
```

鎴栵細

```bash
docker compose up -d --build
```

璇存槑锛?- 瀹瑰櫒榛樿璇诲彇 `/app/configs/config.yaml`
- 濡傛灉浣犳寕杞戒簡 `./configs:/app/configs`锛屽涓绘満鐩綍閲屽繀椤绘湁 `config.yaml`

### 鏂瑰紡浜岋細椋炵墰 / NAS 鐩存帴鎷夐暅鍍?
鏇存帹鑽愪紭鍏堜娇鐢?Docker Hub 闀滃儚锛涘緢澶?NAS 瀵?`docker.io` 鐨勬媺鍙栭€熷害閫氬父姣?`ghcr.io` 鏇寸ǔ瀹氥€?
#### Docker Hub

```yaml
services:
  nodes-check:
    image: troywww/nodes-check:latest
    container_name: nodes-check
    ports:
      - "18808:18808"
    volumes:
      - ./configs:/app/configs
      - ./runtime:/app/runtime
    restart: unless-stopped
    command: ["/app/nodes-check", "-config", "/app/configs/config.yaml"]
```

#### GHCR

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

璇存槑锛?- 杩欎袱绉嶆柟寮忛兘涓嶉渶瑕?NAS 鐜板満 `build`
- 浣犲彧闇€瑕佸噯澶囪嚜宸辩殑閰嶇疆鐩綍鍜岃繍琛岀洰褰?- 濡傛灉 GHCR 鍖呴粯璁ゆ槸绉佹湁鐨勶紝闇€瑕佸厛鍦?GitHub Packages 閲屾妸瀹冩敼鎴愬叕寮€

## GitHub Actions 鑷姩鍙戝竷闀滃儚

浠撳簱鍐呯疆宸ヤ綔娴侊細
- `.github/workflows/publish-image.yml`

瑙﹀彂鏂瑰紡锛?- push 鍒?`main`
- push `v*` 鏍囩
- 鎵嬪姩瑙﹀彂 `workflow_dispatch`

榛樿鍙戝竷锛?- `ghcr.io/troywww/nodes-check:latest`

濡傛灉浣犻厤缃簡浠撳簱 Secrets锛?- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

杩樹細鍚屾鍙戝竷鍒帮細
- `docker.io/troywww/nodes-check:latest`

## Docker Hub 閰嶇疆鏂规硶

鍦?GitHub 浠撳簱 `Settings -> Secrets and variables -> Actions` 閲屾柊澧烇細

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

鍏朵腑锛?- `DOCKERHUB_USERNAME` 鏄綘鐨?Docker Hub 鐢ㄦ埛鍚?- `DOCKERHUB_TOKEN` 寤鸿浣跨敤 Docker Hub Access Token锛屼笉瑕佺洿鎺ョ敤瀵嗙爜

閰嶇疆濂藉悗锛岄噸鏂?push 涓€娆?`main`锛屾垨鎵嬪姩杩愯涓€娆?`publish-image` 宸ヤ綔娴佸嵆鍙€?
## 璇存槑

- `final_ips.txt` 鏄渶缁?KV 鍐呭鏉ユ簮鏂囦欢
- 鍘嗗彶姹犱繚瀛樺湪 `runtime/cache/history_valid_nodes.txt`
- 鑻ユ湰杞ǔ瀹氭睜涓虹┖锛屽巻鍙叉睜涓嶄細琚竻绌?- 鑻ユ煇涓繍钀ュ晢鍒嗙被涔熼厤缃繘 KV锛屽垯浼氫紭鍏堝啓瀵瑰簲鍩熷悕鑰屼笉鏄８ IP

## 镜像体积说明

- Linux 镜像只包含 Linux 版 xray。
- Windows 版 xray.exe 不会进入 Docker Linux 镜像。
- 镜像内只保留运行必需的 config.yaml、subscription_urls.txt 和 Linux xray 文件。
- untime 目录在镜像中只创建空目录，实际运行数据建议通过卷挂载保存。
