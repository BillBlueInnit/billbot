# BillBot Hermes 容器镜像

这个目录放的是 Hermes 容器镜像的构建材料，不是镜像本体。

适合弱网用户分发的是构建好的镜像 tar 包，例如：

```text
billbot-hermes-amd64.tar
billbot-hermes-arm64.tar
```

## 网络好的机器：构建并导出镜像

amd64：

```bash
docker build --platform linux/amd64 -t billbot-hermes:latest -f container/hermes/Dockerfile .
docker save billbot-hermes:latest -o billbot-hermes-amd64.tar
```

arm64：

```bash
docker build --platform linux/arm64 -t billbot-hermes:latest -f container/hermes/Dockerfile .
docker save billbot-hermes:latest -o billbot-hermes-arm64.tar
```

Dockerfile 会在构建阶段访问 Debian 软件源和 Hermes 上游安装脚本。大陆网络环境下这里可能失败，建议在网络好的机器或 CI 上构建。

## 网络差的机器：导入镜像

把对应架构的 tar 包放到机器上，然后执行：

```bash
docker load -i billbot-hermes-amd64.tar
docker images billbot-hermes
```

导入成功后，BillBot 默认 Docker sandbox 会直接使用 `billbot-hermes:latest`，不需要再构建镜像。

## 架构说明

- x64/amd64 机器使用 `billbot-hermes-amd64.tar`。
- arm64 机器使用 `billbot-hermes-arm64.tar`。
- Windows 和 Linux 都可以使用 Docker 的 Linux 容器模式，但 CPU 架构要匹配。

## 模型凭据

镜像不内置 API key。把 Hermes 需要的模型供应商凭据写到 env 文件，例如：

```text
OPENAI_API_KEY=...
```

然后在 BillBot 配置里传给 Docker：

```yaml
security:
  sandbox_backend: docker
  sandbox_docker_image: billbot-hermes:latest
  sandbox_docker_args: ["--env-file", "./hermes.env"]
```
