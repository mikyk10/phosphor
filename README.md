# wisp-ai

AI pipeline microservice for [WiSP](https://github.com/mikyk10/wisp) (Waveshare e-Ink Smart Photo frame).

Generates images, applies style transfers, and tags photos via configurable multi-stage LLM pipelines. Designed to run alongside WiSP as an HTTP image source.

## API

| Endpoint | Method | Description |
|---|---|---|
| `/image` | GET | Text-to-image generation |
| `/image` | POST | Image-to-image processing (body: source image) |
| `/tag` | POST | Image tagging (body: image, response: JSON tags) |
| `/health` | GET | Health check |

### Query Parameters

| Param | Endpoints | Description |
|---|---|---|
| `pipeline` | all | Pipeline name from service.yaml (default: `generate`/`remix`/`tag`) |
| `width` | `/image` | Output width in pixels |
| `height` | `/image` | Output height in pixels |
| `orientation` | `GET /image` | `landscape` or `portrait` (maps to size if width/height omitted) |
| `quality` | `/image` | `low`, `medium`, or `high` |
| `max_tags` | `/tag` | Maximum tags to return (default: 10) |

### Examples

```sh
# Generate an image
curl -o art.png "http://localhost:8082/image?pipeline=generate&quality=high"

# Style transfer (img2img)
curl -X POST -H "Content-Type: image/jpeg" --data-binary @photo.jpg \
  -o styled.png "http://localhost:8082/image?pipeline=remix"

# Tag an image
curl -X POST -H "Content-Type: image/jpeg" --data-binary @photo.jpg \
  "http://localhost:8082/tag"
# → {"tags":["sunset","bridge","river"]}
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

Multiple pipelines can be defined and selected via `?pipeline=name`.

## Prompt Files

Prompts use YAML frontmatter for LLM provider/model selection:

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
| `{{.config.Width}}` | Requested width |
| `{{.config.Height}}` | Requested height |
| `{{.config.Orientation}}` | Requested orientation |
| `{{.config.Quality}}` | Requested quality |
| `{{.config.MaxTags}}` | Max tags (tagging only) |

### API Types

| `api_type` | Use |
|---|---|
| `chat` | Text generation (default) |
| `image_generation` | Text-to-image (`/v1/images/generations`) |
| `image_edit` | Image-to-image (`/v1/images/edits`) |

## Integration with WiSP

WiSP's HTTP catalog fetches images from wisp-ai via background cache:

```yaml
# WiSP service.yaml
catalog:
  - key: ai-art
    type: http
    http:
      url: http://wisp-ai:8082/image?pipeline=generate&orientation=landscape
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
  ↓
store/       Execution history (SQLite, in-memory default)
```

## License

MIT
