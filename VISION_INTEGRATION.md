# Z.AI Vision Integration Architecture

## Overview

This implementation replaces direct PDF upload to LLMs with a more accurate **vision-based approach**:

1. **PDF → Images**: Convert each PDF page to a PNG image
2. **Vision Analysis**: Send each image to Z.AI Vision API for detailed content extraction
3. **LLM Synthesis**: Combine all page analyses and use OpenAI to generate final output

## Why This Approach?

- **Better Accuracy**: Vision models can see diagrams, formatting, tables, and visual layout
- **Native Support**: Most vision APIs are designed for images, not PDFs
- **No Size Limits**: Images can be optimized, whereas PDFs have strict size limits
- **Per-Page Processing**: Each page is analyzed individually before synthesis

## Implementation Details

### New Services

#### 1. `PDFService` (internal/services/pdf.go)

- **`ConvertPDFPagesToImages(path)`**: Converts PDF pages to base64-encoded PNG images
- Returns array of `PDFPageImage` with page number and data URI
- Uses `github.com/ledongthuc/pdf` for PDF reading

#### 2. `ZAIVisionService` (internal/services/vision.go)

- **`AnalyzeImage(ctx, imageDataURI, prompt)`**: Analyzes single image
- **`AnalyzeImages(ctx, imageDataURIs, prompt)`**: Analyzes multiple images sequentially
- Direct HTTP client to Z.AI Vision API
- Based on `@z_ai/mcp-server` implementation
- Supports both Z.AI (https://api.z.ai) and ZhipuAI (https://open.bigmodel.cn)

### Modified Services

#### 3. `AIService` (internal/services/ai.go)

- **Constructor updated**: Now accepts Z.AI credentials and PDFService
- **`ExtractExamTopics`**: Changed from `(ctx, []byte)` to `(ctx, string)` (path)
  - If Z.AI configured: uses `extractExamTopicsWithVision()`
  - Otherwise: falls back to OpenAI direct upload
- **`GenerateFlashcards`**: Similar pattern as above
- **New methods**:
  - `extractExamTopicsWithVision()`: Per-page vision analysis + LLM synthesis
  - `generateFlashcardsWithVision()`: Per-page vision analysis + LLM synthesis

#### 4. `IngestionService` (internal/services/ingestion.go)

- Updated to pass PDF path instead of bytes to AIService methods

### Configuration

#### 5. `Config` (internal/config/config.go)

Added new fields:

- `ZAIKey`: Z.AI API key (from `Z_AI_API_KEY`)
- `ZAIBaseURL`: API endpoint (from `Z_AI_BASE_URL`)
- `ZAIModel`: Vision model name (from `Z_AI_VISION_MODEL`)

#### 6. `main.go` (cmd/server/main.go)

Updated AIService initialization to pass all new parameters

## API Flow

### Exam Document Processing

```
PDF Upload
    ↓
ConvertPDFPagesToImages()
    ↓
For each page:
    ↓
    AnalyzeImage(page, "extract exam topics...")
    ↓
Combine page analyses
    ↓
OpenAI LLM synthesizes JSON topics
    ↓
Store in database
```

### Information Document Processing

```
PDF Upload
    ↓
ConvertPDFPagesToImages()
    ↓
Get focus concepts from DB
    ↓
For each page:
    ↓
    AnalyzeImage(page, "extract learnable content...")
    ↓
Combine page analyses
    ↓
OpenAI LLM generates flashcards JSON
    ↓
Store cards in database
```

## Z.AI Vision API Details

Based on the MCP server code analysis:

### Request Format

```json
{
  "model": "glm-4.5v",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "image_url",
          "image_url": {
            "url": "data:image/png;base64,..."
          }
        },
        {
          "type": "text",
          "text": "Your prompt here"
        }
      ]
    }
  ],
  "thinking": { "type": "enabled" },
  "stream": false,
  "temperature": 0.8,
  "top_p": 0.6,
  "max_tokens": 16384
}
```

### Headers

- `Authorization: Bearer {API_KEY}`
- `Content-Type: application/json`
- `X-Title: Flash-AI Vision`
- `Accept-Language: en-US,en`

### Response Format

```json
{
  "choices": [
    {
      "message": {
        "content": "Analysis result..."
      }
    }
  ]
}
```

## Environment Variables

```bash
# Required for vision processing
Z_AI_API_KEY=your_key_here
Z_AI_BASE_URL=https://open.bigmodel.cn/api/paas/v4/
Z_AI_VISION_MODEL=glm-4.5v

# Required for synthesis
OPENAI_API_KEY=your_openai_key
OPENAI_MODEL=gpt-4o-mini
```

## Fallback Behavior

The system gracefully degrades:

1. **Both configured**: Uses Z.AI Vision + OpenAI synthesis (best)
2. **Only OpenAI**: Falls back to direct PDF upload (if model supports)
3. **Neither**: Returns `ErrAIUnavailable`

## Benefits

✅ **Accurate**: Vision models see actual page layout  
✅ **Flexible**: Works with any PDF content (text, images, diagrams)  
✅ **Scalable**: Each page processed independently  
✅ **Robust**: Fallback to direct upload if vision unavailable  
✅ **Transparent**: Progress logging for multi-page documents

## Future Improvements

- Add progress callbacks for UI feedback
- Implement caching for analyzed pages
- Support parallel page processing
- Add retry logic with exponential backoff
- Implement external rendering tools (pdftoppm, ImageMagick) for better image quality
