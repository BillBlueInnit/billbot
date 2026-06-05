# BillBot

License: LGPL-3.0-only

BillBot 是一个跨平台 bot 框架原型，用 Go 后端和静态 dashboard 管理 QQ connector、Hermes runner、prompt、owner 和安全模式。

## 设计原则

- 仓库不内置个人信息、owner QQ、identity、API key 或默认模型。
- NapCatQQ 作为外部 connector 依赖，不随 BillBot 分发源码、二进制、镜像或修改版包。
- 所有运行配置通过 dashboard 或外部配置文件填写。
- 核心后端使用 Go，目标支持 Windows、Linux 和 macOS。

## 当前 v0.1 能力

- Go HTTP 后端。
- `/api/health`
- `/api/config` GET/POST
- `/api/connectors/status`
- `/api/bridge/status`
- `/api/bridge/start`
- `/api/bridge/stop`
- 静态 dashboard 配置页和 bridge 启停控制。
- 配置文件持久化。
- NapCat OneBot HTTP 状态检查和消息发送。
- NapCat OneBot WebSocket 消息接收与事件解析。
- Hermes CLI runner 基础集成。
- 简单 session state JSON 持久化模块。

## 运行

```powershell
D:\golang\go\bin\go.exe run .\cmd\billbot --port 2006
```

然后打开：

```text
http://127.0.0.1:2006
```

也可以指定配置文件：

```powershell
D:\golang\go\bin\go.exe run .\cmd\billbot --config .\config.example.yaml --port 2006
```

## NapCatQQ 连接

BillBot 默认使用 external mode：用户自行安装并启动 NapCatQQ，然后在配置中填写 OneBot HTTP/WebSocket 地址。

默认地址：

```yaml
napcat:
  http: http://127.0.0.1:3000
  ws: ws://127.0.0.1:3001
```

Bridge 启动后会从 WebSocket 接收消息，调用 Hermes，并通过 NapCat HTTP 将回复发回私聊或群聊。

## 开发验证

```powershell
D:\golang\go\bin\gofmt.exe -w .\cmd .\internal
D:\golang\go\bin\go.exe test ./...
D:\golang\go\bin\go.exe build -o $env:TEMP\billbot-check.exe .\cmd\billbot
```

## 合规文档

- `LICENSE`
- `docs/LICENSING.md`
- `THIRD_PARTY_NOTICES.md`

## 致谢

- Thanks to Nous Research and Hermes Agent contributors.
- Thanks to NapCatQQ project authors and contributors.
- Thanks to gorilla/websocket contributors.
- Thanks to go-yaml contributors.
