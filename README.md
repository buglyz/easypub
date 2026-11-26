# go-easypub

![CI](https://github.com/buglyz/easypub/actions/workflows/ci.yml/badge.svg)
![Release](https://github.com/buglyz/easypub/actions/workflows/release.yml/badge.svg)
![Docker](https://github.com/buglyz/easypub/actions/workflows/docker.yml/badge.svg)
将 TXT 转换为 EPUB 的工具，用 Go 重写自 [EasyPub](http://sourceforge.net/projects/easypub/)（v1.50）。
目标：与原工具生成的 EPUB 在结构与样式上**字节级对齐**，支持 Linux/Windows 跨平台、零运行时依赖、单文件分发，并自带 WebUI。

## 核心功能

- TXT → EPUB 2.0
  - `mimetype`（不压缩）+ `META-INF/container.xml`
  - `OEBPS/` 下完整生成 `content.opf`、`toc.ncx`、`style.css`、`cover.html`、`book-toc.html`、`chapterN.html`
- **三种使用形态**：
  - 命令行（CLI）：单文件二进制，脚本/批量场景
  - WebUI：`easypub serve`，浏览器里拖拽上传、配参、识别章节、生成下载
  - Docker：镜像默认启动 WebUI（`0.0.0.0:8080`）
- 自动识别章节标题（中文小说"第N章"、"序"、"楔子"、"番外"等，英文 `Chapter N`）
- 支持三类正则来源：完整正则 / 简易三段式正则 / 预定义附加正则
- 编码自动检测：UTF-8(BOM/无 BOM)、UTF-16(BE/LE BOM)、GBK、GB18030、Big5
- 中文排版 CSS（参考老牛样式），参数可调：行高、字号、段间距、对齐、缩进
- 段落首行全角空格缩进保留
- 空行去除 / 段间加空行
- 阅读器字体路径（ereaders.xml），支持 Nook、SONY PRS-505、老牛方案等
- 可选调用本机 `kindlegen` 生成 MOBI（桌面/服务器）

## 平台支持

| 平台 | 架构 | 状态 |
|---|---|---|
| Linux | amd64 | ✅ 纯静态二进制（CGO_ENABLED=0） |
| Linux | arm64 | ✅（树莓派、ARM 服务器、Termux 均可） |
| Windows | amd64 | ✅ |
| macOS | amd64 | ✅（`make darwin` / Release / CI 交叉编译） |
| macOS | arm64 | ✅（`make darwin` / Release / CI 交叉编译） |

Android 不在本项目支持范围。

## 安装

**方式一：直接下载预编译二进制**

到 [Releases](https://github.com/buglyz/easypub/releases) 下载对应平台的二进制：
- `easypub-linux-amd64` / `easypub-linux-arm64`
- `easypub-windows-amd64.exe`
- `easypub-darwin-amd64` / `easypub-darwin-arm64`
- `checksums-sha256.txt` 校验和

每个 Release 的二进制由 GitHub Actions 在打 `v*` tag 时自动构建并发布。

**方式二：从源码构建**

需要 **Go 1.25+**（见 `go.mod`）。

```bash
go build -o easypub ./cmd/easypub
# Windows:
go build -o easypub.exe ./cmd/easypub
```

或用 Makefile 交叉编译全部平台：

```bash
make all      # dist/ 下产出 linux/windows/darwin 五平台
make linux    # 仅 Linux
make windows  # 仅 Windows
make darwin   # 仅 macOS
```

## 命令行用法

最简：

```bash
./easypub -i book.txt
# 产出 book.epub（与输入同目录、同名）
```

完整：

```bash
./easypub -i book.txt -o out.epub \
  --title "书名" --author "作者" \
  --config config.xml --ereaders ereaders.xml --font 0 \
  --mobi
```

**批量转换（服务器/脚本场景）：**

```bash
# 转换 /data/books 目录内全部 *.txt/*.utf8/*.gbk 到 /data/epub
./easypub -i /data/books -o /data/epub

# 幂等重跑：已生成的 EPUB 不重复转换（适合 cron）
./easypub -i /data/books -o /data/epub --skip-existing

# cron 示例（每 10 分钟转换新放入的文件）：
# */10 * * * * /opt/easypub/easypub -i /data/books -o /data/epub --skip-existing --quiet
```

| 选项 | 默认 | 说明 |
|---|---|---|
| `-i` | （必填） | 输入 TXT 路径**或目录**（目录时批量转换其中 `.txt/.utf8/.gbk`） |
| `-o` | 输入同名 `.epub` | 输出 EPUB 路径；目录输入时表示**输出目录**（默认=输入目录） |
| `--title` | TXT 首行或文件名 | 书名（批量时应用到所有文件） |
| `--author` | 空 | 作者 |
| `--config` | 自动尝试 `config.xml` | EasyPub 风格 config.xml 路径 |
| `--ereaders` | 随 config 目录 | ereaders.xml 路径 |
| `--font` | 0 (-1=用第一个) | ereaders.xml 中阅读器字体方案索引 |
| `--mobi` | false | 同时调用 `kindlegen` 生成 MOBI |
| `--skip-existing` | false | 批量模式：目标 EPUB 已存在时跳过（幂等） |
| `-j` | 1 | 批量模式：并发转换数（`0` 按 CPU 核数） |
| `--quiet` | false | 静默模式 |
| `-version` | | 打印版本 |
## WebUI 用法

```bash
./easypub serve
# 默认仅本机: http://127.0.0.1:8080
# 局域网/公网 + 鉴权: ./easypub serve -addr 0.0.0.0:8080 -token mysecret
# 令牌也可用环境变量: EASYPUB_TOKEN=mysecret ./easypub serve -addr 0.0.0.0:8080
```

| 选项 | 默认 | 说明 |
|---|---|---|
| `-addr` | `127.0.0.1:8080` | 监听地址；默认仅本机。公网/局域网请显式指定并注意无鉴权风险 |
| `-dir`  | 当前工作目录 | 工作目录，用于查找 `config.xml`/`ereaders.xml`，也是默认输出目录 |
| `-token` | 空（不鉴权） | 访问令牌；非空时所有 `/api/*` 端点要求令牌（`X-Auth-Token` 头 / `Authorization: Bearer <token>` / `?token=`），前端静态壳正常加载并在启动时检测弹窗，`/healthz` 免鉴权；也支持环境变量 `EASYPUB_TOKEN` |

> 启用令牌后前端会在收到 401 时弹出令牌输入框，记住后自动带上；下载链接自动追加 `?token=`。
WebUI 流程：
1. 拖拽或点击选择 TXT 文件（自动探测编码）
2. 填写书名、作者（可留空自动推断）
3. 设置分章方式（自动识别 / 自定义正则 / 按字数 / 整本一章），点「识别章节」预览目录
4. 可选：删除空行、段间加空行
5. 调整排版参数（行高 / 字号 / 段前距 / 缩进 / 对齐），可选同时生成 MOBI
6. 点「开始转换」，完成后直接下载 EPUB（及 MOBI）

生成的文件默认保存到 `<工作目录>/.easypub-output/`。

## Docker 用法

镜像默认启动 **WebUI**，监听 `0.0.0.0:8080`。工作目录为 `/data`（可挂载以持久化输出与 `config.xml`）。

**拉取并运行（GHCR）：**

```bash
docker run --rm -p 8080:8080 ghcr.io/buglyz/easypub:edge
# 浏览器打开 http://127.0.0.1:8080
```

**挂载数据目录：**

```bash
mkdir -p data
docker run --rm -p 8080:8080 -v "$PWD/data:/data" ghcr.io/buglyz/easypub:edge
```

**docker compose：**

```bash
docker compose up -d --build
```

**本地构建：**

```bash
make docker          # 或 docker build -t easypub:local .
make docker-run
```

**镜像标签：**

| 标签 | 说明 |
|---|---|
| `edge` | `master` 分支最新构建 |
| `latest` / `x.y.z` | 打 `v*` tag 时由 Release 同步推送 |
| `sha-xxxxxxx` | 对应提交短 SHA |

镜像由 GitHub Actions（`.github/workflows/docker.yml`）构建并推送到 `ghcr.io/buglyz/easypub`，支持 `linux/amd64` 与 `linux/arm64`。

> 注意：镜像默认**无鉴权**。公网暴露请务必通过 `EASYPUB_TOKEN` 环境变量设置访问令牌（`docker run -e EASYPUB_TOKEN=xxx ...` 或 compose 里的 `EASYPUB_TOKEN`），或加反向代理鉴权。容器内默认非 root 运行；MOBI 仍依赖宿主机/镜像内是否提供 `kindlegen`（官方镜像未内置）。

## Linux 服务器部署

三种方式任选：**systemd 直接跑二进制**（推荐，轻量可控）、**Docker**（隔离干净）、**批量 CLI**（脚本/cron 自动化）。

### 方式一：systemd 直接部署（推荐）

```bash
# 1. 构建 Linux 二进制（在开发机上）
make linux           # 产物 dist/easypub-linux-amd64

# 2. 上传到服务器后一键安装
scp dist/easypub-linux-amd64 root@your-server:/tmp/
ssh root@your-server
cd /tmp && git clone <本仓库> easypub   # 或仅拷贝 deploy/ 目录
sudo ./deploy/install.sh /tmp/easypub-linux-amd64

# 3. 设置访问令牌并重启
sudo sed -i 's/^Environment=EASYPUB_TOKEN=.*/Environment=EASYPUB_TOKEN='$(openssl rand -hex 24)'/' /etc/systemd/system/easypub.service
sudo systemctl daemon-reload && sudo systemctl restart easypub
```

- 服务默认监听 `127.0.0.1:8080`，建议用 Nginx/Caddy 反向代理加 HTTPS；
- 数据目录 `/opt/easypub`（输出 `.easypub-output/`、可选 `config.xml`）；
- 查看日志：`journalctl -u easypub -f`；
- 安全加固已内置（nonroot 用户、`ProtectSystem=strict`、`PrivateTmp`、`NoNewPrivileges` 等）。

### 方式二：Docker

```bash
docker run -d --name easypub -p 8080:8080 \
  -e EASYPUB_TOKEN="$(openssl rand -hex 24)" \
  -v "$PWD/data:/data" --restart unless-stopped \
  ghcr.io/buglyz/easypub:edge
```

### 方式三：批量 CLI + cron

适合"把 txt 丢进目录自动出 epub"的无人值守场景：

```bash
# 每 10 分钟转换一次新增文件（已生成的自动跳过）
*/10 * * * * /opt/easypub/easypub -i /data/books -o /data/epub --skip-existing --quiet

# 手动跑一次并看汇总
/opt/easypub/easypub -i /data/books -o /data/epub
# 输出: OK a.txt -> /data/epub/a.epub (编码 UTF-8, 120 章)
# 输出: 批量完成: 成功 2 / 失败 0 / 跳过 1 / 共 3
```

> 批量模式单文件失败不中断，退出码非 0 表示有文件失败，可配合脚本告警。

## 配置文件

可直接沿用原 EasyPub 的 `config.xml` / `ereaders.xml`。示例见 [`configs/`](configs/)。

`config.xml` 中已支持的选项：

- `RecentOptions.lineheight` / `fontsize` / `margintop` / `textalign` / `indent`：CSS 排版参数
- `RecentOptions.full_reg` / `simple_reg_p1/p2/p3` / `simple_reg_ext` / `simple_reg_leadingspace`：章节切分正则
- `RecentOptions.automark`：自动识别
- `RecentOptions.removeblankline` / `addspace` / `addspacecount`：段落处理
- `MyRegExp.AdditionalReg` / `FullReg`：预定义正则下拉
- `AdvancedOptions.kindlegenexe` / `kindlegencompress` / `kindlegenoption`：MOBI 生成参数
- `AdvancedOptions.forceemptychapter`：保留空章节

未支持的项（如字体嵌入子集、HTML 原始标签透传等冷门 GUI 项）会被忽略而不报错，详见 `internal/config/config.go`。

## MOBI 输出（仅桌面/服务器）

`kindlegen` 是亚马逊专有、已停止维护的工具，**只有 Windows/macOS/Linux x86 三种版本，没有 ARM/Android 版**。
本项目不重写其内部实现，仅调用。把 `kindlegen_v2.9.exe`（或 Linux/mac 上对应的可执行）放到：

1. `bin/` 子目录（与原工具一致），或
2. 输入文件同目录，或
3. 系统 `PATH`。

加 `--mobi` 后工具会自动定位并调用。Linux 上需自行下载并赋予执行权限的 `kindlegen` Linux x86 二进制。

## 与原工具对齐情况

以原 EasyPub v1.50 生成的样例 epub（含 350 章正文）为基准，下列文件**字节级一致**：

- `mimetype`
- `META-INF/container.xml`
- `OEBPS/style.css`（在默认配置下）
- `OEBPS/cover.html`（给定同书名同作者时）
- 各 `OEBPS/chapterN.html` 模板部分（XHTML 头、generator 元、CSS 引用、`titlel2std` 标题、`.a` 段落格式）

唯一可能不一致的：`dc:identifier`（UID）我们用随机生成，样例固定为某个；这是 EPUB 规范要求每书唯一，是设计差异而非 bug。

## 目录结构

```
go-easypub/
├── cmd/easypub/         CLI 入口（支持 serve / 批量目录转换）
├── internal/
│   ├── batch/           目录批量转换（服务器/脚本场景）
│   ├── config/          config.xml / ereaders.xml 解析
│   ├── converter/       转换核心，CLI 与 WebUI 共用
│   ├── css/             style.css 模板生成
│   ├── txt/             TXT 解析与分章
│   ├── epub/            EPUB 2.0 打包
│   ├── mobi/            kindlegen 调用
│   ├── util/            编码探测、转义等
│   └── webui/           WebUI 服务（内嵌静态前端）
├── configs/             原版兼容的示例 config.xml / ereaders.xml
├── css/                 提取自原工具的样例 style.css
├── deploy/              Linux 部署：systemd 服务单元 + 一键安装脚本
├── testdata/            测试样例 TXT
├── Dockerfile           多阶段构建，默认入口为 WebUI
├── docker-compose.yml   一键启动 WebUI（支持 EASYPUB_TOKEN）
├── Makefile             交叉编译与常用任务
└── build.sh             POSIX shell 构建脚本（同 Makefile 逻辑的备份）
```

## 测试

```bash
make test    # 或 go test ./...
```

覆盖：分章（多正则、中阿数字、空行、加空行、缩进、HTML 转义），CSS 模板参数，EPUB 结构与 zip 方法，BOM 头，编码探测（UTF-8/GBK/UTF-16LE），config 往返，converter 集成，批量转换（多文件/输出目录/幂等跳过/单文件失败不中断），WebUI（HTTP 上传/转换/下载/路径穿越防护/令牌鉴权/探活豁免）。

## 持续集成

- **CI**（`.github/workflows/ci.yml`）：每次 push 到 `master` 或 PR 触发，跑 `go vet` + `go test` + 多平台构建冒烟。
- **Release**（`.github/workflows/release.yml`）：打 `v*` tag 触发，跑测试后构建 linux/windows/darwin 五个二进制 + sha256 校验和，自动创建 GitHub Release 并上传。
- **Docker**（`.github/workflows/docker.yml`）：`master` / `v*` tag / 手动触发时构建 `linux/amd64` + `linux/arm64` 镜像并推送到 GHCR；PR 仅构建不推送。
发版示例：

```bash
git tag v0.1.0
git push origin v0.1.0
# Actions 跑完即可在 Releases 页看到产物
```
