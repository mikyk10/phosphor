<p align="center">
  <img src="docs/logo.svg" width="120" height="120" alt="phosphor logo">
</p>

<h1 align="center">phosphor</h1>

<p align="center">
  AI image pipeline microservice — chain LLM calls, Lua scripts, and headless Chrome rendering into configurable, YAML-defined pipelines.
</p>

<p align="center">
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#configuration">Configuration</a> &middot;
  <a href="#prompt-files">Prompt Files</a> &middot;
  <a href="#architecture">Architecture</a>
</p>

---

> **Experimental.** This project is a work in progress. The design may change significantly, and nothing is guaranteed to work. Use at your own risk.

Generates and transforms images through configurable multi-stage pipelines. Each stage can call an LLM, run a Lua script for data gathering, or render HTML to PNG via headless Chrome. Originally built as an image source for [WiSP](https://github.com/mikyk10/wisp) (Waveshare e-Ink Smart Photo frame), but works standalone as a general-purpose AI image pipeline.

## API

| Endpoint | Method | Description |
|---|---|---|
| `/pipeline/:name` | GET | Execute a named pipeline (no source image) |
| `/pipeline/:name` | POST | Execute a named pipeline (body: source image) |
| `/health` | GET | Health check |
| `/metrics` | GET | Prometheus metrics |

`/metrics` exposes `phosphor_pipeline_executions_total{pipeline,status}`,
`phosphor_pipeline_duration_seconds{pipeline}` and
`phosphor_stage_failures_total{stage,error_code}` alongside the standard Go
runtime collectors.

### Query Parameters

| Param | Description |
|---|---|
| `size` | Output size, e.g. `1024x1536`, `800x480` |
| `quality` | `low`, `medium`, or `high` |
| `max_tags` | Maximum tags to return (tagging pipelines) |

### Examples

```sh
# Generate an AI image
curl -o art.png "http://localhost:8082/pipeline/generate?size=1024x1024&quality=high"

# Style transfer (img2img)
curl -X POST -H "Content-Type: image/jpeg" --data-binary @photo.jpg \
  -o styled.png "http://localhost:8082/pipeline/remix"

# Render a live dashboard (Lua fetches weather data, Chrome renders HTML)
curl -o dashboard.png "http://localhost:8082/pipeline/dashboard?size=800x480"

# AI dinner suggestion (Lua + LLM + Chrome render, 3-stage pipeline)
curl -o dinner.png "http://localhost:8082/pipeline/dinner?size=800x480"
```

## Quick Start

```sh
go run . web run
```

That is the whole setup: when `config/config.yaml` or `config/service.yaml` is
absent, phosphor falls back to the `*.yaml.example` file next to it (and logs a
warning saying so). To customise, copy the templates and edit — a real file
always wins over the example:

```sh
cp config/config.yaml.example config/config.yaml
cp config/service.yaml.example config/service.yaml

# LLM pipelines need a key; the dashboard pipeline does not.
export OPENAI_API_KEY=sk-...
go run . web run
```

The `dashboard` pipeline uses only Lua + Chrome rendering and works without an API key:

```sh
curl -o dashboard.png "http://localhost:8082/pipeline/dashboard?size=800x480"
```

### Docker

```sh
docker compose up --build
```

The Docker image includes Chromium for headless HTML rendering. The dev
container runs [air](https://github.com/air-verse/air) for hot reload, driven
by `.air.toml` — edits to `.go`, `config.yaml` and `service.yaml` rebuild and
restart automatically.

### Releases

Pushing a semver tag publishes a versioned image to GHCR:

```sh
git tag v0.2.0 && git push origin v0.2.0
# -> ghcr.io/mikyk10/phosphor:0.2.0 and :0.2
```

`latest` continues to track `main`.

## Configuration

Two YAML files in `config/`:

### config.yaml — Infrastructure

```yaml
port: 8082
log_level: DEBUG
max_request_body_mb: 20   # POST bodies above this are rejected with 413
max_concurrent_pipelines: 4   # simultaneous pipeline executions (0 = unlimited)

# auth:
#   token: "${PHOSPHOR_TOKEN}"   # optional shared token for /pipeline routes

database:
  dsn: ":memory:"       # in-memory (default) or file path for debug
  max_history: 200      # keep only the newest N pipeline executions (-1 disables pruning)
  store_image_outputs: false  # persist generated image blobs in the history DB

ai:
  request_timeout_sec: 120
  max_retries: 3
  providers:
    openai:
      endpoint: https://api.openai.com/v1
      api_key: "${OPENAI_API_KEY}"
```

When `auth.token` is set, every `/pipeline` request must present it as an
`Authorization: Bearer <token>` header or a `?key=<token>` query parameter —
clients that can only be given a URL (WiSP's HTTP catalog, for example) can
just append `?key=...`. `/health` is never protected. Leaving `auth.token`
unset keeps the endpoints open, as before.

### service.yaml — Pipelines

```yaml
pipelines:
  # Text-to-image via LLM
  generate:
    defaults:
      size: 1536x1024
      quality: low
    stages:
      - name: brainstorm
        output: text
        prompt: config/prompts/examples/generate/meta.md
      - name: render
        output: image
        prompt: config/prompts/examples/generate/image.md

  # Image-to-image style transfer
  remix:
    defaults:
      quality: low
    stages:
      - name: stylize
        output: image
        prompt: config/prompts/examples/remix/stylize.md
        image_input: _source

  # Photo tagging
  tag:
    defaults:
      max_tags: 10
    stages:
      - name: descriptor
        output: text
        prompt: config/prompts/examples/tag/descriptor.md
        image_input: _source
      - name: tagger
        output: text
        prompt: config/prompts/examples/tag/tagger.md

  # Live weather dashboard (no LLM needed)
  dashboard:
    defaults:
      size: 800x480
    stages:
      - name: data
        output: text
        prompt: config/prompts/examples/dashboard_data.md
      - name: render
        output: image
        prompt: config/prompts/examples/dashboard_render.md

  # AI dinner suggestion (LLM + weather context)
  dinner:
    defaults:
      size: 800x480
    stages:
      - name: context
        output: text
        prompt: config/prompts/examples/dinner_data.md
      - name: suggest
        output: text
        prompt: config/prompts/examples/dinner_suggest.md
      - name: render
        output: image
        prompt: config/prompts/examples/dinner_render.md
```

More example pipelines (`proverbs`, `surreal`, `nordic`) are available in `config/service.yaml.example`.

Multiple pipelines can be defined and selected via `GET|POST /pipeline/{name}`.

## Prompt Files

Each stage references a prompt file with YAML frontmatter:

```markdown
---
provider: openai
model: gpt-4o
api_type: chat
temperature: 0.7
---
Describe this photo in detail.
```

### API Types

| `api_type` | Use | Provider needed |
|---|---|---|
| `chat` | LLM text generation (default) | Yes |
| `image_generation` | Text-to-image (`/v1/images/generations`) | Yes |
| `image_edit` | Image-to-image (`/v1/images/edits`) | Yes |
| `lua` | Run a Lua script for data gathering | No |
| `render` | HTML template → headless Chrome screenshot | No |

### Lua Scripts (`api_type: lua`)

Lua stages run embedded scripts with built-in modules:

```lua
local json = require("json")   -- JSON encode/decode
local http = require("http")   -- HTTP client

local raw = http.get("https://api.open-meteo.com/v1/forecast?...")
local data = json.decode(raw)

return json.encode({
  temperature = tostring(data.current.temperature_2m),
  humidity = tostring(data.current.relative_humidity_2m),
})
```

### HTML Rendering (`api_type: render`)

Render stages take HTML (inline or from a file path) and capture a PNG screenshot via headless Chrome. The viewport size is controlled by the `size` parameter.

A single headless Chrome process is shared by the whole service (each render is just a new tab), and concurrent renders are capped at 2 — override with the `RENDER_MAX_CONCURRENT` environment variable.

Previous stage outputs are available as template variables:

```html
{{$d := json .prev.output}}
<div class="value">{{index $d "temperature"}}°C</div>
```

### Template Variables

| Variable | Description |
|---|---|
| `{{.prev.output}}` | Previous stage text output |
| `{{.stages.NAME.output}}` | Named stage text output |
| `{{json .prev.output}}` | Parse previous output as JSON map |
| `{{.config.Size}}` | Requested size |
| `{{.config.Quality}}` | Requested quality |
| `{{.config.MaxTags}}` | Max tags (tagging only) |

## Integration with [WiSP](https://github.com/mikyk10/wisp)

WiSP's HTTP catalog fetches images from phosphor:

```yaml
# WiSP service.yaml
catalog:
  - key: ai-art
    type: http
    http:
      url: http://phosphor:8082/pipeline/generate?size=1024x1536
      cache:
        type: background
        depth: 10

  - key: dashboard
    type: http
    http:
      url: http://phosphor:8082/pipeline/dashboard?size=800x480
      cache:
        type: realtime
```

## Architecture

```
handler/     Echo v5 HTTP handlers
  ↓
usecase/     Business logic + PipelineRunner
  ↓
pipeline/    StageExecutor interface, result types
  ↓
llm/         LLM providers (OpenAI compatible)
lua/         Lua script executor (gopher-lua)
render/      HTML → PNG via headless Chrome (chromedp)
  ↓
store/       Execution history (SQLite, in-memory default)
```

Execution history is trimmed in-process: a background pruner keeps only the
newest `database.max_history` pipeline executions (and their steps/outputs),
running once at startup and every 5 minutes thereafter. Generated image blobs
are not persisted unless `database.store_image_outputs` is enabled, since the
default DSN is in-memory and blobs would otherwise grow RAM without bound.

## License

[MIT](LICENSE)
