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
# 1. Copy config templates
cp config/config.yaml.example config/config.yaml
cp config/service.yaml.example config/service.yaml

# 2. Set your OpenAI API key and run
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

The Docker image includes Chromium for headless HTML rendering.

## Configuration

Two YAML files in `config/`:

### config.yaml — Infrastructure

```yaml
port: 8082
log_level: DEBUG

database:
  dsn: ":memory:"       # in-memory (default) or file path for debug

ai:
  request_timeout_sec: 120
  max_retries: 3
  providers:
    openai:
      endpoint: https://api.openai.com/v1
      api_key: "${OPENAI_API_KEY}"
```

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

## License

[MIT](LICENSE)
