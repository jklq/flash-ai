# Flash AI

Flash AI is a Go web app that blends the FSRS spaced repetition algorithm with AI-assisted content extraction. Upload your reference PDFs and past exams, then review an automatically prioritised queue of flashcards that evolve with your study progress.

## Features

- 🔁 **Review queue backed by FSRS** – every answer updates stability, difficulty, and due dates using [go-fsrs](https://github.com/open-spaced-repetition/go-fsrs).
- 🤖 **AI flashcard generation** – information PDFs are summarised through OpenAI to create atomic, high-quality cards in bulk.
- 📊 **Exam-weighted insights** – past exams surface the most frequent concepts so the system can bias future flashcards toward likely test content.
- 📂 **SQLite persistence** – cards, reviews, documents, concepts, and topics live in a self-contained SQLite database.
- 🌐 **Web UI** – review cards, upload PDFs, and inspect topics from a friendly frontend.

## Prerequisites

- Go 1.24.1 or newer (the module uses the Go 1.24 toolchain to satisfy dependency requirements).
- An OpenAI API key with access to the model specified in `OPENAI_MODEL` (defaults to `gpt-4o-mini`).
- `sqlite3` is not required; the application bundles a pure Go driver.

## Quick start

```bash
cp .env.example .env             # add your OpenAI credentials
go run ./cmd/server              # launches the web server on :8080
```

Then open [http://localhost:8080](http://localhost:8080) and:

1. Visit **Upload PDFs** to add either information or exam files.
2. Navigate to **Review** to work through the FSRS queue.
3. Check **Key Topics** for exam-weighted concept rankings.

Uploaded files are stored under `static/uploads/`; the SQLite database defaults to `data/flashcards.db`. You can customise these paths via environment variables (see below).

## Configuration

Environment variables (or `.env`) control runtime behaviour:

| Variable            | Default                                 | Description                                                                       |
| ------------------- | --------------------------------------- | --------------------------------------------------------------------------------- |
| `OPENAI_API_KEY`    | _(none)_                                | OpenAI API key for fallback LLM synthesis (used after vision analysis).           |
| `OPENAI_MODEL`      | `gpt-4o-mini`                           | Any compatible OpenAI chat completion model for text-based synthesis.             |
| `Z_AI_API_KEY`      | _(none)_                                | **Z.AI API key for vision analysis** - extracts content from PDF pages as images. |
| `Z_AI_BASE_URL`     | `https://open.bigmodel.cn/api/paas/v4/` | Z.AI API endpoint (use `https://api.z.ai/api/paas/v4/` for Z.AI platform).        |
| `Z_AI_VISION_MODEL` | `glm-4.5v`                              | Vision model for analyzing PDF page images.                                       |
| `DATABASE_PATH`     | `./data/flashcards.db`                  | Location of the SQLite database.                                                  |
| `UPLOAD_DIR`        | `./static/uploads`                      | Folder for persisted PDF uploads.                                                 |
| `PORT`              | `8080`                                  | HTTP port for the server.                                                         |

### Vision-Based PDF Processing

The system now uses **Z.AI Vision API** to process PDFs by:

1. Converting each PDF page to an image
2. Sending each image to Z.AI Vision API for analysis
3. Combining the page analyses
4. Using OpenAI LLM to synthesize the final flashcards or topics

This approach provides **better accuracy** than trying to send entire PDFs to the model, as the vision model can see the actual visual layout, diagrams, and formatting.

**To use vision-based processing:**

- Set `Z_AI_API_KEY` to your Z.AI API key
- Set `OPENAI_API_KEY` for the final synthesis step
- If `Z_AI_API_KEY` is not set, the system falls back to direct PDF upload (if supported by your OpenAI model)

**To get a Z.AI API key:**

- For Z.AI platform: https://z.ai/model-api
- For ZhipuAI (China): https://bigmodel.cn

## API snapshot

| Method & Path                | Description                                                                                   |
| ---------------------------- | --------------------------------------------------------------------------------------------- | ---- | ---- | -------------------------------------------------- |
| `GET /api/cards/next`        | Returns the next due card (or `null` when the queue is empty).                                |
| `POST /api/cards/:id/review` | Body `{ "rating": "again                                                                      | hard | good | easy" }`; records a review and updates scheduling. |
| `POST /api/documents`        | Multipart endpoint that accepts one or more PDFs plus a `docType` of `information` or `exam`. |
| `GET /api/topics`            | Lists concepts ordered by exam-derived weight.                                                |
| `GET /api/health`            | Simple health probe.                                                                          |

Responses are JSON; the frontend uses the same endpoints via `fetch`.

## Project layout

```
cmd/server        # Application entrypoint
internal/api      # Gin HTTP handlers and routing
internal/config   # Environment loading
internal/db       # SQLite connection + schema migrations
internal/models   # Shared data structures and FSRS bridging
internal/services # Domain services: flashcards, AI, PDFs, ingestion
static/           # Web pages, styles, and uploaded files
```

## Development workflow

- Run `go build ./...` to ensure the backend compiles.
- `gofmt -w ./cmd ./internal` keeps Go sources formatted.
- Frontend assets live in `static/`; no extra build step is required.
- AI operations require network access. Without a valid API key the upload endpoint still accepts files but returns an error payload indicating the missing configuration.

Enjoy studying smarter! Contributions and new ideas are welcome.
