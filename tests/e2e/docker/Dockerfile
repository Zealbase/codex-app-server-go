# Dockerfile for the codex-go SDK e2e test image.
#
# Codex CLI is distributed as an npm package, so this image uses Node.js.
# Pin CODEX_VERSION to match the host CLI for protocol compatibility.
#
# Build:
#   docker build -t codex-e2e:local .
#   docker build --build-arg CODEX_VERSION=0.136.0-alpha.2 -t codex-e2e:local .
#
# The test suite mounts ~/.codex at /root/.codex:ro at runtime so the container
# inherits the logged-in auth token — no `codex login` step needed inside Docker.
#
# Run standalone (manual debug):
#   docker run --rm -i \
#     -v ~/.codex:/root/.codex:ro \
#     -e OPENAI_API_KEY=$OPENAI_API_KEY \
#     codex-e2e:local

FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
        git \
        ripgrep \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG CODEX_VERSION=0.136.0-alpha.2

RUN npm install -g @openai/codex@${CODEX_VERSION} \
    && codex --version

RUN mkdir -p /root/.codex

WORKDIR /workspace
RUN git init .

ENTRYPOINT ["codex"]
# Default: run app-server in stdio JSON-RPC mode.
CMD ["app-server", "--stdio"]
