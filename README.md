# CosAvLink

cosplay.jav.pw 最新视频浏览 + javdb.com 磁力链接自动查询桌面工具。

基于 [Wails v2](https://wails.io/) (Go + React) 构建，运行于 Windows 桌面。

## 功能

- 自动抓取 cosplay.jav.pw 视频列表，自适应填充整个窗口
- 自动提取番号并在 javdb.com 查询磁力链接
- 无番号时使用视频标题模糊搜索 javdb
- 主磁力区无结果时自动从短评区提取用户分享的磁力
- 点击视频即可查看磁力，无需额外操作
- 浅色主题，流畅动画

## 环境要求

- Windows 10/11（WebView2 Runtime，通常已预装）
- 本机安装的 Chrome 或 Edge（go-rod 会自动查找）
- [Go 1.21+](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)：
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

## 运行

```bash
# 开发模式
wails dev

# 构建生产版本
wails build
# 产物在 build/bin/CosAvLink.exe
```

程序启动后会自动启动一个隐藏的浏览器实例用于访问 javdb.com（绕过 Cloudflare）。
首次查询时浏览器会自动初始化，之后复用同一实例。

## 目录结构

```
main.go                      Wails 入口 + 窗口配置
app.go                       Go 绑定（暴露给前端的函数）
internal/
  model/                     数据结构
  cache/                     泛型 TTL 缓存
  code/                      番号提取
  cosplay/                   cosplay.jav.pw 分页抓取
  browser/                   go-rod 浏览器管理（Cloudflare 绕过）
  javdb/                     javdb 磁力查找
frontend/
  src/                       React 前端源码
  wailsjs/                   Wails JS 绑定（build 时自动生成）
wails.json                   Wails 项目配置
```

## 免责声明

仅用于个人学习与技术研究。所抓取的内容与磁力链接由对应第三方网站负责，使用者需自行承担相应责任并遵守当地法律与各网站的服务条款。
