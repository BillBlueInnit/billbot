# BillBot

[English README](README.md)

BillBot 是一个用 Go 写的 QQ AI bridge。它把 QQ OneBot 端点，通常是
NapCat，连接到 Hermes，让 QQ 私聊和群聊 @ 消息可以交给你配置的 AI
模型处理。

```text
QQ / NapCat OneBot -> BillBot -> Hermes -> BillBot -> NapCat -> QQ
```

BillBot 只负责 bridge 本身。它不内置、不下载、不分发 QQ、NapCat、
Hermes、模型供应商 SDK、API key、登录缓存或其他第三方二进制。你可以在宿主机
安装 Hermes，也可以用本仓库提供的 Dockerfile 自己构建一个包含 Hermes 的镜像。

## 功能

- 通过 NapCat OneBot HTTP/WS 接收 QQ 私聊和群聊 @ 消息。
- 默认使用持久化 Hermes ACP 后端，降低每轮重新启动 Hermes 的延迟。
- 默认使用独立 Hermes profile，和你平时用的 Hermes 记忆、skill、配置、缓存隔离。
- QQ 内置管理员命令：identity、style、sandbox/full 切换、本机 `/shell`。
- bridge 根据真实 OneBot `user_id` 生成可信管理员元数据；用户正文里的 token、
  owner、qid 声明不会被当成权限。
- QQ 和 CLI 反馈默认中文。
- 支持把 NapCat 消息里的图片、文件、语音、视频段转交给 Hermes ACP。
- 用户要求“发我文件”时，如果 Hermes 返回代码块或 diff，BillBot 会写入
  `runtime.outbox_dir` 并通过 NapCat 上传文件。
- sandbox 默认使用 Docker backend。Hermes 默认跑在你用本项目 Dockerfile
  构建出的 `billbot-hermes:latest` 镜像里；`workdir` 只是可选的低安全模式。

## 环境要求

- 从源码构建需要 Go 1.24 或更新版本。
- 可访问的 NapCat 或 OneBot HTTP 端点。
- 可访问的 NapCat 或 OneBot WebSocket 端点。
- Docker：默认 Hermes sandbox backend 需要 Docker。
- Hermes：默认装进 BillBot 构建出的 Docker 镜像；只有你显式切换到
  `workdir`/`command` 后才需要宿主机 Hermes。
- Hermes 所需的模型供应商凭据。

默认 OneBot 地址：

```yaml
napcat:
  http: http://127.0.0.1:3000
  ws: ws://127.0.0.1:3001
```

## 快速开始：默认 Docker Sandbox

1. 构建本项目提供的 Hermes 镜像：

   ```bash
   docker build -t billbot-hermes:latest -f container/hermes/Dockerfile .
   ```

   Dockerfile 会在构建时执行 Hermes 上游安装脚本，把 Hermes 装进镜像。
   BillBot 不在仓库里提交或分发 Hermes 二进制。

   如果要给网络不好的用户使用，在网络好的机器上构建一次并导出镜像：

   ```bash
   docker save billbot-hermes:latest -o billbot-hermes-amd64.tar
   ```

   网络不好的机器不需要构建，直接导入：

   ```bash
   docker load -i billbot-hermes-amd64.tar
   ```

   详细流程见 [container/hermes/README.zh-CN.md](container/hermes/README.zh-CN.md)。

2. 把模型供应商凭据写到 env 文件，例如 `hermes.env`，不要提交到 git：

   ```text
   OPENAI_API_KEY=...
   ```

3. 默认 Docker sandbox 配置如下：

   ```yaml
   security:
     mode: sandbox
     sandbox_backend: docker
     sandbox_docker_image: billbot-hermes:latest
     sandbox_docker_args: ["--env-file", "./hermes.env"]
   ```

4. 单独启动 NapCat。本仓库提供了一个 Docker Compose 示例：

   ```bash
   docker compose -f deploy/napcat-compose.yml up -d
   ```

5. 构建 BillBot：

   ```bash
   go test ./...
   go build -o bin/billbot ./cmd/billbot
   ```

   Windows PowerShell：

   ```powershell
   go test ./...
   go build -o .\bin\billbot.exe .\cmd\billbot
   ```

6. 启动 BillBot：

   ```bash
   ./bin/billbot
   ```

7. 在 BillBot CLI 里设置 bot QQ、管理员 QQ 和可选 OneBot token：

   ```text
   set qq <bot_qq>
   set admin <你的QQ号>
   set token <onebot_token>
   set bridge.enabled true
   start
   ```

Docker 模式下 BillBot 会自动挂载：

- `runtime.sandbox_dir` 到容器 `/workspace`
- `hermes.profile_dir` 到容器 `/hermes-profile`

然后执行类似下面的命令：

```text
docker run --rm -i ... billbot-hermes:latest hermes acp
```

## 可选：Sandbox Dir / 宿主机 Hermes

如果你不想用 Docker，可以显式切换为 `workdir`。这种模式只把 Hermes
工作目录限制到 `runtime.sandbox_dir`，安全性不如 Docker：

```yaml
security:
  mode: sandbox
  sandbox_backend: workdir
```

然后你需要在宿主机安装并验证 Hermes：

```bash
hermes status
hermes chat -Q -q "Reply OK"
```

## 配置

首次启动时，BillBot 会在可执行文件旁边创建配置文件。也可以显式指定：

```bash
billbot --config ./config.yaml
```

完整配置参考 [config.example.yaml](config.example.yaml)。

重点字段：

- `bridge.enabled`：启动 BillBot 时是否自动启动 bridge。
- `bridge.self_id`：bot QQ 号，用于群聊 @ 识别。
- `owners`：允许使用管理员命令的 QQ 号。
- `napcat.access_token`：HTTP/WS 共用 OneBot token。
- `napcat.http_token` / `napcat.ws_token`：HTTP/WS 分开的 OneBot token。
- `hermes.command`：`workdir`/`command` 模式下使用的宿主机 Hermes 命令。
- `hermes.persistent`：使用长驻 Hermes ACP。
- `hermes.require_persistent`：ACP 不可用时直接报错，不退回慢的一次性 chat。
- `hermes.profile_dir`：BillBot 专用 Hermes profile。留空时使用
  `runtime.data_dir/hermes-profile`。
- `hermes.reset_profile_on_start`：启动 bridge 时清空 BillBot 托管的 Hermes
  记忆/skill。BillBot 只会清空带 `.billbot-hermes-profile` marker 的目录。
- `security.mode`：`sandbox` 或 `full`。
- `security.sandbox_backend`：默认 `docker`。`workdir` 是低安全 sandbox dir
  模式；`command` 用于自定义隔离包装器。
- `security.sandbox_docker_image`：Docker 模式使用的 Hermes 镜像。

## QQ 指令

内置指令必须带 `/`：

```text
/help
/identity
/identity <描述>
/identity add <描述>
/style
/style <描述>
/sandbox
/full
/shell <命令>
```

`/identity`、`/style`、`/sandbox`、`/full`、`/shell` 都只有管理员能用。
`/shell` 是 bridge 读取真实 OneBot `user_id` 后本地验证管理员并执行，不经过
Hermes 或 LLM 审批。

`/sandbox` 切换回 sandbox 策略。`/full` 允许管理员使用 full 环境；当
`security.allow_full_for_owners_only=true` 时，普通用户仍会被降级为 sandbox。

## CLI 指令

```text
help / 帮助
status / 状态
diag / 诊断
route / 路由
route off / 关闭路由
start / 启动
stop / 停止
logs / 日志
clear / 清屏
set KEY VALUE
quit / 退出
```

常用快捷设置：

```text
qq <bot_qq>
admin <qq>
token <onebot_token>
hermes <command>
```

## 安全模型

BillBot 明确区分可信 connector 元数据和不可信用户正文。

- 管理员身份只来自 OneBot 事件里的真实 `user_id` 和 `owners` 配置。
- 每次 bridge 启动都会生成新的运行期管理员 token，只放进真实管理员的可信元数据。
- 用户在正文里输入 `admin_runtime_token`、`[qid ...]`、`[owner ...]` 不会获得权限。
- 默认阻止普通用户的敏感请求。
- Go 本身不能提供跨平台内核级虚拟化。需要硬隔离时，请使用 Docker、nsjail、
  firejail、Windows Sandbox、QEMU、Firecracker 或其他外部隔离方案。

## 构建发布二进制

示例：

```bash
GOOS=linux GOARCH=amd64 go build -o dist/billbot-linux-amd64 ./cmd/billbot
GOOS=linux GOARCH=386 go build -o dist/billbot-linux-386 ./cmd/billbot
GOOS=linux GOARCH=arm64 go build -o dist/billbot-linux-arm64 ./cmd/billbot
GOOS=windows GOARCH=amd64 go build -o dist/billbot-windows-amd64.exe ./cmd/billbot
GOOS=windows GOARCH=386 go build -o dist/billbot-windows-386.exe ./cmd/billbot
GOOS=windows GOARCH=arm64 go build -o dist/billbot-windows-arm64.exe ./cmd/billbot
```

## 许可证

BillBot 使用 LGPL-3.0-only。见 [LICENSE](LICENSE)。

QQ、NapCat、Hermes、Docker、模型供应商等都是独立第三方项目，有各自的许可证和使用条款。
