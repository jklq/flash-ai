# Testing the Z.AI Vision Integration

## Setup

1. **Copy the example environment file:**

   ```bash
   cp .env.example .env
   ```

2. **Add your API keys to `.env`:**

   ```bash
   # Required for vision analysis
   Z_AI_API_KEY=your_zai_api_key_here

   # Required for synthesis
   OPENAI_API_KEY=your_openai_api_key_here
   ```

3. **Choose the correct Z.AI platform:**
   - For Z.AI (international): `Z_AI_BASE_URL=https://api.z.ai/api/paas/v4/`
   - For ZhipuAI (China): `Z_AI_BASE_URL=https://open.bigmodel.cn/api/paas/v4/`

## Running the Server

```bash
go run ./cmd/server
```

The server will start on port 8080 (or the port specified in `PORT` env var).

## Testing with a PDF

1. **Open the web interface:**

   ```
   http://localhost:8080
   ```

2. **Upload a PDF:**

   - Click "Upload PDFs"
   - Choose a PDF file
   - Select document type:
     - **Exam**: For extracting exam topics and frequencies
     - **Information**: For generating flashcards from learning material

3. **Monitor the logs:**
   You should see output like:
   ```
   Converting PDF to images for vision analysis...
   Processing 5 pages with Z.AI Vision API...
   Analyzing page 1/5...
   Analyzing page 2/5...
   ...
   Synthesizing topics from all pages...
   ```

## Expected Behavior

### With Z.AI Vision Configured

1. PDF is converted to images (one per page)
2. Each page is sent to Z.AI Vision API for analysis
3. Vision API returns detailed text description of each page
4. All page analyses are combined
5. OpenAI synthesizes the final output (topics or flashcards)
6. Results are stored in the database

### Without Z.AI Vision (Fallback)

1. PDF is base64-encoded
2. Entire PDF is sent directly to OpenAI
3. OpenAI processes the document (if supported by model)
4. Results are stored in the database

## Troubleshooting

### Error: "vision api error: status=401"

- Check your `Z_AI_API_KEY` is correct
- Verify the API key is active and has credits

### Error: "vision api error: status=404"

- Check your `Z_AI_BASE_URL` is correct
- Verify you're using the right platform (Z.AI vs ZhipuAI)

### Error: "openai integration is not configured"

- Ensure both `Z_AI_API_KEY` and `OPENAI_API_KEY` are set
- The vision API needs OpenAI for the final synthesis step

### Slow Processing

- Multi-page PDFs process one page at a time
- Large PDFs may take several minutes
- Check server logs for progress updates

## Verifying Results

1. **For Exam Documents:**

   - Go to "Key Topics" page
   - You should see extracted topics with frequencies
   - Topics should reflect content from all PDF pages

2. **For Information Documents:**
   - Go to "Review" page
   - You should see generated flashcards
   - Cards should cover concepts from throughout the document

## API Testing

You can also test via direct API calls:

```bash
# Upload a PDF
curl -X POST http://localhost:8080/api/documents \
  -F "files=@your-document.pdf" \
  -F "docType=information"

# Check topics
curl http://localhost:8080/api/topics

# Get next flashcard
curl http://localhost:8080/api/cards/next
```

## Performance Notes

- Each page takes ~2-5 seconds to analyze with vision API
- A 10-page PDF might take 30-60 seconds total
- Progress is logged to stderr
- Consider the rate limits of your Z.AI API plan

## Cost Considerations

- Z.AI Vision API charges per image
- Each PDF page = 1 API call
- Check your API plan limits before processing large documents
- OpenAI synthesis is a single call regardless of page count
