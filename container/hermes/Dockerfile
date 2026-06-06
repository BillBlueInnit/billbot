# syntax=docker/dockerfile:1

FROM debian:bookworm-slim

ARG HERMES_INSTALL_URL="https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh"

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH="/root/.local/bin:/root/.cargo/bin:/root/.bun/bin:/root/.npm-global/bin:/usr/local/bin:/usr/bin:/bin"

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		bash \
		ca-certificates \
		curl \
		ffmpeg \
		git \
		openssh-client \
		procps \
		python3 \
		python3-pip \
		python3-venv \
		ripgrep \
		unzip \
		xz-utils \
	&& rm -rf /var/lib/apt/lists/*

RUN set -eux; \
	curl -fsSL "$HERMES_INSTALL_URL" -o /tmp/hermes-install.sh; \
	bash /tmp/hermes-install.sh; \
	rm -f /tmp/hermes-install.sh; \
	command -v hermes

RUN mkdir -p /workspace /hermes-profile

WORKDIR /workspace

ENV HOME=/hermes-profile
ENV USERPROFILE=/hermes-profile
ENV XDG_CONFIG_HOME=/hermes-profile/config
ENV XDG_DATA_HOME=/hermes-profile/data
ENV XDG_CACHE_HOME=/hermes-profile/cache
ENV HERMES_HOME=/hermes-profile
ENV BILLBOT_HERMES_PROFILE_DIR=/hermes-profile
