# clem samples

Self-contained Dockerised setup for trying clem without touching your host.

Samples today:

- [`anthropic/`](anthropic/) - **Discord** coordination + [Anthropic Claude API](https://www.anthropic.com/). Bring your own API key; no local GPU needed. Published as `ghcr.io/jahwag/clem-sample:latest`.
- [`ollama-nemotron-4b/`](ollama-nemotron-4b/) - **Discord** coordination + local [NVIDIA Nemotron 3 Nano 4B](https://ollama.com/library/nemotron-3-nano) via Ollama. 2.8 GB, fits an 8 GB MacBook Air, emits proper tool_use blocks. Published as `ghcr.io/jahwag/clem-sample:latest-ollama`.
- [`slack-nemotron-4b/`](slack-nemotron-4b/) - **Slack** coordination, same local model. Uses [korotovsky/slack-mcp-server](https://github.com/korotovsky/slack-mcp-server). Published as `ghcr.io/jahwag/clem-sample:latest-slack`.
- [`github-tasks/`](github-tasks/) - **GitHub Issues** coordination via `gh` CLI (no chat MCP). Task board lives in a dedicated repo with `clem:*` labels.

## Quick start (prebuilt image)

No clone required. Pull and run the latest published image:

```bash
docker run --rm -it --privileged \
  -e DISCORD_TOKEN=... -e DISCORD_SERVER_ID=... \
  -p 7681:7681 \
  ghcr.io/jahwag/clem-sample:latest
```

Images are published to GHCR on every release tag for `linux/amd64` and `linux/arm64`.

## Build

From the repo root (to customise the sample):

```bash
docker build -f samples/Dockerfile --build-arg SAMPLE=ollama-nemotron-4b -t clem-nemotron .
```

Substitute `podman build` if that's what you have.

## Run

See the sample's README for full instructions. Both `docker` and `podman` work. Two modes:

- **Tour** - interactive shell; explore `clem init` / `clem vault` without real credentials.
- **Runtime** - systemd-enabled; `clem provision` creates OS users and starts agents. Needs `--privileged` on docker or `--systemd=always` on podman.

## Building your own sample

Any directory under `samples/` with a `clem.yaml` can be built the same way. Drop a new folder in, point `SAMPLE=` at it, rebuild.
