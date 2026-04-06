# nodes-check

`nodes-check` 鏄竴涓潰鍚戣蒋璺敱鍦烘櫙鐨勪唬鐞嗚妭鐐圭瓫閫変笌 Cloudflare 鍙戝竷宸ュ叿銆?
鏍稿績娴佺▼锛?- 浠庤闃呬腑鍙彁鍙?`IP / 绔彛 / 鍚嶇О`
- 鍚堝苟鍘嗗彶鎴愬姛姹?- TCP 棰勭瓫
- 涓よ疆 `xray-core` 鐪熷欢杩熸祴璇?- 鎸夊垎绫诲拰鎺ㄩ€侀厤缃敓鎴愭渶缁堢粨鏋?- 鎺ㄩ€佸埌 Cloudflare Worker/KV 涓?Cloudflare DNS

## 鍔熻兘姒傝

- 鍐呯疆 `xray-core` 鐪熷欢杩熸祴璇?- WebUI + token 鐧诲綍淇濇姢
- 鏀寔澶氭潯璁㈤槄閾炬帴
- 鏀寔瀹氭椂鑷姩杩愯
- 鏀寔鍘嗗彶鎴愬姛姹犲鐢?- 鏀寔 Cloudflare Worker/KV 鎺ㄩ€?- 鏀寔 Cloudflare DNS 鎺ㄩ€?- 鏀寔 Docker 閮ㄧ讲
- 鏀寔閫氳繃 GitHub 鑷姩鍙戝竷 GHCR 闀滃儚

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
## 閰嶇疆璇存槑

浠撳簱涓殑 [config.yaml](D:\project\nodes_check\configs\config.yaml) 鏄劚鏁忓悗鐨勯粯璁ら厤缃紝鍙互鐩存帴浣滀负妯℃澘浣跨敤銆?
閮ㄧ讲鍓嶈嚦灏戦渶瑕佷慨鏀硅繖浜涘唴瀹癸細
- `web.auth_token`
- `probe.template.*`
- `cloudflare.worker.*`
- `cloudflare.dns.*`

涓嶈鎶婄湡瀹?token銆佸煙鍚嶃€乁UID 鍐嶆彁浜ゅ洖浠撳簱銆?
## 鏈湴杩愯

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

### 鏈湴婧愮爜鏋勫缓

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
- 濡傛灉鎸傝浇浜?`./configs:/app/configs`锛屽涓绘満鐩綍閲屽繀椤绘湁 `config.yaml`

### 椋炵墰 / NAS 鐩存帴鎷?GHCR 闀滃儚

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

璇存槑锛?- 杩欑鏂瑰紡涓嶉渶瑕?NAS 鐜板満 `build`
- 浣犲彧闇€瑕佸噯澶囪嚜宸辩殑 `configs` 鍜?`runtime` 鐩綍
- 濡傛灉 GHCR 鍖呴粯璁ゆ槸绉佹湁鐨勶紝闇€瑕佸厛鍦?GitHub Packages 閲屾妸瀹冩敼鎴愬叕寮€

## 闀滃儚璇存槑

- Linux 闀滃儚鍙寘鍚?Linux 鐗?`xray`
- Windows 鐗?`xray.exe` 涓嶄細杩涘叆 Docker Linux 闀滃儚
- 闀滃儚鍐呭彧淇濈暀杩愯蹇呴渶鏂囦欢
- `runtime` 鐩綍鍦ㄩ暅鍍忎腑鍙垱寤虹┖鐩綍锛屽疄闄呰繍琛屾暟鎹缓璁€氳繃鍗锋寕杞戒繚瀛?
## 璇存槑

- `final_ips.txt` 鏄渶缁?KV 鍐呭鏉ユ簮鏂囦欢
- 鍘嗗彶姹犱繚瀛樺湪 `runtime/cache/history_valid_nodes.txt`
- 鑻ユ湰杞ǔ瀹氭睜涓虹┖锛屽巻鍙叉睜涓嶄細琚竻绌?- 鑻ユ煇涓繍钀ュ晢鍒嗙被涔熼厤缃繘 KV锛屽垯浼氫紭鍏堝啓瀵瑰簲鍩熷悕鑰屼笉鏄８ IP