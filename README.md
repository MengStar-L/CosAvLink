# CosAvLink

一个 Go 服务 + 网页：抓取 [cosplay.jav.pw](https://cosplay.jav.pw/) 的**最新视频**（名称 + 封面），
点击某个视频时**实时**去 [javdb.com](https://javdb.com/) 查询该番号的**磁力链接**。

## 工作原理

1. `cosplay.jav.pw` 是公开的 WordPress 站点，普通 HTTP 请求即可抓取列表（`net/http` + goquery）。
2. 从标题（如 `[DSAM-002]`、`CME-003`）或封面文件名提取**番号**。无番号的同人/cosplay 条目会标记为「无番号」。
3. 点击「获取磁力」→ 后端通过 **FlareSolverr** 自动解决 javdb 的 Cloudflare 验证 →
   `/search?q=番号&f=all` → 详情页 `/v/xxx` → 解析 `#magnets-content` 的磁力链接。
4. 结果带 **TTL 缓存**（命中 12h / 无结果 1h / 被拦截 2min），并用 `singleflight` 去重并发请求。

## 运行

需要 Go 1.26+，以及 [Docker](https://docs.docker.com/get-docker/)（用于运行 FlareSolverr）。

```bash
# 1) 启动 FlareSolverr（自动解决 Cloudflare 验证）
docker compose up -d

# 2) 启动服务
go run ./cmd/server
# 打开 http://localhost:8080
```

> 首次点击「获取磁力」时，FlareSolverr 会自动解决 Cloudflare 验证，可能需要等待 30-60 秒。

## 配置（环境变量）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 端口 |
| `FLARESOLVERR_URL` | `http://localhost:8191/v1` | FlareSolverr API 地址 |
| `MAX_PARALLEL` | `2` | 并发请求数上限 |

```bash
# 例：自定义端口 + 远程 FlareSolverr
PORT=9000 FLARESOLVERR_URL=http://my-server:8191/v1 go run ./cmd/server
```

## 关于 Cloudflare

javdb 使用 Cloudflare 的 **managed challenge（Turnstile，即 "Just a moment…" 页面）**。
本项目通过 [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr) **自动解决**这些验证——
FlareSolverr 使用内置的 undetected Chrome 浏览器，能够在大多数情况下自动通过 Cloudflare 的人机检测。

只需确保 FlareSolverr 服务正在运行（`docker compose up -d`），程序会自动调用其 API 获取页面内容。
如果被拦截，缓存会在 2 分钟后自动过期，下次请求会重新尝试。

> 注意：如果你的网络/IP 被 Cloudflare 判定风险极高，FlareSolverr 也可能无法通过验证。
> 此时可尝试更换 IP 或使用代理。

## 登录与可见性

- 普通有码片（DSAM/MIMK/CME 等，cosplay 的主力）设置 `over18=1` cookie 即可**免登录**看到磁力。
- **FC2 / 部分无码**资源需要登录 javdb 才可见，这类条目可能显示为空。当前版本不登录。

## 目录结构

```
cmd/server/main.go         入口：配置、HTTP 服务、优雅关闭
internal/model/            Video / Magnet 数据结构
internal/code/             番号提取（标题正则 + 封面文件名兜底）
internal/cosplay/          cosplay.jav.pw 列表抓取（net/http + goquery）
internal/flaresolverr/     FlareSolverr HTTP 客户端
internal/javdb/            javdb 磁力抓取（缓存 + singleflight）
internal/server/           HTTP handler + 内嵌网页模板
docker-compose.yml         FlareSolverr Docker 部署
```

## 免责声明

仅用于个人学习与技术研究。所抓取的内容与磁力链接由对应第三方网站负责，使用者需自行承担相应责任并遵守当地法律与各网站的服务条款。
