# wisp-ai

> **Experimental.** This project is a work in progress. The design may change significantly, and nothing is guaranteed to work. Use at your own risk.

Image pipeline microservice for [WiSP](https://github.com/mikyk10/wisp) (Waveshare e-Ink Smart Photo frame).

Generates and transforms images through configurable multi-stage pipelines. Stages can include LLM calls, Lua scripts for data gathering, and headless Chrome rendering for HTML-to-image conversion. Designed to run alongside WiSP as an HTTP image source.

## API

| Endpoint | Method | Description |
|---|---|---|
| `/pipeline/:name` | GET | Execute a named pipeline (no source image) |
| `/pipeline/:name` | POST | Execute a named pipeline (body: source image) |
| `/health` | GET | Health check |

### Query Parameters

| Param | Description |
|---|---|
| `size` | Output size, e.g. `1024x1536` |
| `quality` | `low`, `medium`, or `high` |
| `max_tags` | Maximum tags to return (tagging pipelines) |

### Examples

```sh
# Generate an image
curl -o art.png "http://localhost:8082/pipeline/generate?size=1024x1024&quality=high"

# Style transfer (img2img)
curl -X POST -H "Content-Type: image/jpeg" --data-binary @photo.jpg \
  -o styled.png "http://localhost:8082/pipeline/remix"

# Render a dashboard
curl -o dashboard.png "http://localhost:8082/pipeline/dashboard?size=800x480"
```

## Quick Start

```sh
# 1. Copy config templates
cp config/config.yaml.example config/config.yaml
cp config/service.yaml.example config/service.yaml

# 2. Set your API key in config/config.yaml
#    ai.providers.openai.api_key: "${OPENAI_API_KEY}"

# 3. Run
export OPENAI_API_KEY=sk-...
go run . web run
```

### Docker

```sh
docker compose up --build
```

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
  generate:
    defaults:
      width: 1024
      height: 1024
      orientation: landscape
      quality: high
    stages:
      - name: brainstorm
        output: text
        prompt: prompts/example/gen_meta.md
      - name: render
        output: image
        prompt: prompts/example/gen_image.md

  remix:
    defaults:
      quality: low
    stages:
      - name: stylize
        output: image
        prompt: prompts/example/stylize.md
        image_input: _source

  tag:
    defaults:
      max_tags: 10
    stages:
      - name: descriptor
        output: text
        prompt: prompts/example/descriptor.md
        image_input: _source
      - name: tagger
        output: text
        prompt: prompts/example/tagger.md
```

Multiple pipelines can be defined and selected via `GET|POST /pipeline/{name}`.

## Prompt Files

Prompts use YAML frontmatter for provider/model/type selection:

```markdown
---
provider: openai
model: gpt-image-1
api_type: image_generation
---
{{.prev.output}}
```

### Template Variables

| Variable | Description |
|---|---|
| `{{.prev.output}}` | Previous stage text output |
| `{{.stages.NAME.output}}` | Named stage text output |
| `{{.config.Size}}` | Requested size |
| `{{.config.Quality}}` | Requested quality |
| `{{.config.MaxTags}}` | Max tags (tagging only) |
| `{{json .prev.output}}` | Parse previous output as JSON |

### API Types

| `api_type` | Use |
|---|---|
| `chat` | Text generation (default) |
| `image_generation` | Text-to-image (`/v1/images/generations`) |
| `image_edit` | Image-to-image (`/v1/images/edits`) |
| `render` | HTML template → headless Chrome screenshot |
| `lua` | Lua script execution (data gathering) |

## Integration with WiSP

WiSP's HTTP catalog fetches images from wisp-ai:

```yaml
# WiSP service.yaml
catalog:
  - key: ai-art
    type: http
    http:
      url: http://wisp-ai:8082/pipeline/generate?size=1024x1536
      cache:
        type: background
        depth: 10
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

MIT
