# CosAvLink

cosplay.jav.pw 最新视频浏览 + javdb.com 磁力链接自动查询桌面工具。

基于 [Wails v2](https://wails.io/) (Go + React) 构建，运行于 Windows 桌面。

## 功能

- 自动抓取 cosplay.jav.pw 视频列表，16 条/页，支持翻页
- 自动提取番号并在 javdb.com 查询磁力链接
- 无番号时使用视频标题模糊搜索 javdb
- 主磁力区无结果时自动从短评区提取用户分享的磁力
- 磁力自动预取：卡片可见时即开始查询，无需手动点击
- 下一页自动预取，翻页瞬间加载

## 环境要求

- Windows 10/11（WebView2 Runtime，通常已预装）
- [Go 1.21+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)：
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- [FlareSolverr](https://github.com/FlareSolverr/FlareSolverr)（用于自动解决 Cloudflare 验证）

## 运行

```bash
# 1) 启动 FlareSolverr（需要 Docker）
docker run -d --name flaresolverr -p 8191:8191 ghcr.io/flaresolverr/flaresolverr:latest

# 2) 开发模式
wails dev

# 3) 构建生产版本
wails build
# 产物在 build/bin/ 目录
```

## 配置

FlareSolverr 默认连接 `http://localhost:8191/v1`。如需修改，编辑 `app.go` 中的 URL。

## 目录结构

```
main.go                      Wails 入口 + 窗口配置
app.go                       Go 绑定（暴露给前端的函数）
internal/
  model/                     数据结构
  cache/                     泛型 TTL 缓存
  code/                      番号提取
  cosplay/                   cosplay.jav.pw 分页抓取
  flaresolverr/              FlareSolverr HTTP 客户端
  javdb/                     javdb 磁力查找
frontend/
  src/                       React 前端源码
  wailsjs/                   Wails JS 绑定（build 时自动生成）
wails.json                   Wails 项目配置
```

## 免责声明

仅用于个人学习与技术研究。所抓取的内容与磁力链接由对应第三方网站负责，使用者需自行承担相应责任并遵守当地法律与各网站的服务条款。
