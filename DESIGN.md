# wisp-ai — AI Pipeline Microservice Design

## Overview

WiSP から分離した AI パイプライン実行サービス。
画像生成 (text-to-image / img2img) と画像タグ付けを HTTP API で提供する。

WiSP の HTTP catalog (background cache) から呼ばれる想定。

## Architecture

```
cmd/             Cobra CLI (web run)
  ↓
handler/         Echo v5 HTTP ハンドラー (リクエスト解析・レスポンス構築)
  ↓
usecase/         ビジネスロジック (パイプライン選択・実行・結果変換)
  ↓
pipeline/        汎用マルチステージパイプラインランナー
  ↓
llm/             LLM プロバイダー抽象化 (OpenAI 互換)
  ↓
store/           実行履歴 DB (in-memory SQLite デフォルト)
```

## Directory Layout

```
wisp-ai/
├── main.go                    # エントリーポイント, DI コンテナ構築
├── go.mod
├── Dockerfile
│
├── cmd/
│   └── root.go                # Cobra ルート + web run コマンド
│
├── config/
│   ├── config.go              # GlobalConfig + ServiceConfig 構造体
│   ├── loader.go              # YAML 読み込み + 環境変数展開
│   ├── config.yaml.example    # インフラ設定例 (port, DB, providers)
│   └── service.yaml.example   # サービス定義例 (pipelines)
│
├── handler/
│   ├── image.go               # GET /image, POST /image
│   ├── tag.go                 # POST /tag
│   └── health.go              # GET /health
│
├── usecase/
│   ├── generate.go            # 画像生成 (text-to-image)
│   ├── remix.go               # 画像加工 (img2img)
│   └── tag.go                 # タグ付け
│
├── pipeline/
│   ├── stage.go               # StageExecutor interface, StageResult, PipelineResult
│   ├── runner.go              # PipelineRunner (マルチステージ実行)
│   └── runner_test.go
│
├── llm/
│   ├── provider.go            # 4 executor 実装 (chat text/image, image gen/edit)
│   ├── prompt.go              # YAML frontmatter + Go template
│   ├── prompt_test.go
│   └── transport.go           # HTTP transport middleware
│
├── store/
│   ├── model.go               # PipelineExecution, StepExecution, StepOutput
│   ├── repository.go          # Repository interface + GORM 実装
│   └── sqlite.go              # SQLite 接続 (in-memory デフォルト)
│
├── route/
│   └── route.go               # Echo ルーティング登録
│
└── prompts/
    └── example/               # プロンプトテンプレート例
```

## HTTP API

### `GET /image` — Text-to-Image 生成

```
GET /image?pipeline=generate&width=800&height=480&orientation=landscape
```

| Param | Type | Required | Default | Description |
|---|---|---|---|---|
| pipeline | string | N | `generate` | パイプライン名 (config の pipelines キー) |
| width | int | N | — | 出力画像の幅 (サービス側でリサイズ) |
| height | int | N | — | 出力画像の高さ |
| orientation | string | N | — | `landscape` / `portrait` (パイプラインに変数として渡す) |
| quality | string | N | — | `low` / `medium` / `high` (パイプラインに変数として渡す) |

**Response:** `image/*` (JPEG or PNG)

### `POST /image` — Image-to-Image 加工

```
POST /image?pipeline=remix&width=800&height=480&quality=high
Content-Type: image/jpeg

<image bytes>
```

| Param | Type | Required | Default | Description |
|---|---|---|---|---|
| pipeline | string | N | `remix` | パイプライン名 |
| width | int | N | — | 出力画像の幅 |
| height | int | N | — | 出力画像の高さ |
| quality | string | N | — | `low` / `medium` / `high` |

**Request Body:** ソース画像バイナリ (`image/jpeg` or `image/png`)
**Response:** `image/*`

### `POST /tag` — 画像タグ付け

```
POST /tag?pipeline=tag
Content-Type: image/jpeg

<image bytes>
```

**Response:**
```json
{"tags": ["sunset", "bridge", "river"]}
```

### `GET /health`

**Response:** `200 OK`

## Usecase Layer

Handler は直接 PipelineRunner を呼ばない。Usecase を介する。

```go
// usecase/generate.go
type GenerateUsecase interface {
    Run(ctx context.Context, input GenerateInput) (*GenerateOutput, error)
}

type GenerateInput struct {
    PipelineName string
    Width        int
    Height       int
    Orientation  string
    Quality      string
}

type GenerateOutput struct {
    ImageData   []byte
    ContentType string
}
```

```go
// usecase/remix.go
type RemixUsecase interface {
    Run(ctx context.Context, input RemixInput) (*RemixOutput, error)
}

type RemixInput struct {
    PipelineName string
    SourceImage  []byte
    Width        int
    Height       int
}

type RemixOutput struct {
    ImageData   []byte
    ContentType string
}
```

```go
// usecase/tag.go
type TagUsecase interface {
    Run(ctx context.Context, input TagInput) (*TagOutput, error)
}

type TagInput struct {
    PipelineName string
    Image        []byte
}

type TagOutput struct {
    Tags []string
}
```

各 Usecase の内部:
1. Config からパイプライン定義を取得
2. PipelineExecution レコード作成
3. `PipelineRunner.RunPipeline()` 実行 (width/height/orientation は template 変数として渡す)
4. 結果を抽出 (画像 or テキスト→タグパース)
5. PipelineExecution ステータス更新
6. Output 返却

## Configuration

2 ファイルに分離 (WiSP と同じパターン):

### config.yaml — インフラ設定

```yaml
port: 8080
log_level: INFO

database:
  dsn: ":memory:"           # デフォルト: in-memory (ステートレス相当)
  # dsn: ./wisp-ai.db       # デバッグ時: ファイル永続化

ai:
  request_timeout_sec: 120
  max_retries: 3
  providers:
    openai:
      endpoint: https://api.openai.com/v1
      api_key: "${OPENAI_API_KEY}"
    ollama:
      endpoint: http://localhost:11434/v1
```

### service.yaml — パイプライン定義

```yaml
pipelines:
  generate:
    stages:
      - name: brainstorm
        output: text
        prompt: prompts/gen_meta.md
      - name: render
        output: image
        prompt: prompts/gen_image.md

  remix:
    stages:
      - name: stylize
        output: image
        prompt: prompts/stylize.md
        image_input: _source

  tag:
    stages:
      - name: descriptor
        output: text
        prompt: prompts/descriptor.md
        image_input: _source
      - name: tagger
        output: text
        prompt: prompts/tagger.md
```

## Pipeline Validation

エンドポイントはパイプラインの最終ステージ出力型を実行前に検証する:

- `GET /image`, `POST /image` → 最終ステージが `output: image` でなければ `400 Bad Request`
- `POST /tag` → 最終ステージが `output: text` でなければ `400 Bad Request`

パイプライン定義に type フィールドは持たない。config はフラットな map のまま。

## Template Variables

パイプラインのプロンプトテンプレートで使用可能な変数:

```
{{.prev.output}}                — 前ステージのテキスト出力
{{.stages.NAME.output}}         — 名前付きステージのテキスト出力
{{.config.Width}}               — リクエストの width
{{.config.Height}}              — リクエストの height
{{.config.Orientation}}         — リクエストの orientation
```

## DB (実行履歴)

- デフォルト: in-memory SQLite (プロセス終了で消える)
- デバッグ時: ファイル DSN に切り替え
- テーブル: `pipeline_executions`, `step_executions`, `step_outputs` のみ
- GenerationCacheEntry, Tag, ImageTag は **不要** (WiSP 固有)

## DI (uber/dig)

```
dig.Container
  ├── *config.Config
  ├── *gorm.DB
  ├── store.Repository
  ├── *pipeline.PipelineRunner
  ├── usecase.GenerateUsecase
  ├── usecase.RemixUsecase
  ├── usecase.TagUsecase
  ├── handler.ImageHandler
  ├── handler.TagHandler
  └── handler.HealthHandler
```

## Implementation Phases

### Phase 1: Scaffold
- go.mod, main.go, cmd/root.go, config/, store/sqlite.go
- `GET /health` が動く状態

### Phase 2: Pipeline Core
- pipeline/stage.go, pipeline/runner.go (import path 変更)
- llm/ (provider, prompt, transport — ほぼ verbatim)
- store/model.go, store/repository.go
- テスト pass

### Phase 3: Usecase + Handler
- usecase/generate.go, handler/image.go (GET /image)
- route/route.go, DI 接続
- E2E テスト (mock LLM server)

### Phase 4: POST /image + POST /tag
- usecase/remix.go, usecase/tag.go
- handler/image.go (POST), handler/tag.go
- タグパースロジック

### Phase 5: Container
- Dockerfile, docker-compose 統合, config.yaml.example
