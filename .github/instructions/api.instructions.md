---
applyTo: "**"
---

# API Reference - Paperless NGX & LLM Integration

Quick reference for API endpoints and payloads used in the document automation workflow.

## Paperless NGX API

### Get Documents
```http
GET /api/documents/?page_size=100000
Host: your-paperless-domain.com
Authorization: Token YOUR_PAPERLESS_API_TOKEN
```

### Download Document PDF
```http
GET /api/documents/{document_id}/download/
Host: your-paperless-domain.com
Authorization: Token YOUR_PAPERLESS_API_TOKEN
```

### Update Document
```http
PATCH /api/documents/{document_id}/
Host: your-paperless-domain.com
Authorization: Token YOUR_PAPERLESS_API_TOKEN
Content-Type: application/json

{
  "title": "New Title"
}
```

```http
PATCH /api/documents/{document_id}/
Host: your-paperless-domain.com
Authorization: Token YOUR_PAPERLESS_API_TOKEN
Content-Type: application/json

{
  "content": "Extracted text content"
}
```

## LLM API

### Check Available Models
```http
GET /v1/models
```

### Title Generation
```http
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "ibnzterrell/Meta-Llama-3.3-70B-Instruct-AWQ-INT4",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "Return possible titles for this document, the titles should be describe the content of this document.\n\n- Keep the language of the original document.\n- Return this result as a json array.\n- You must respond with only valid JSON.\n- No markdown, only json.\n- In the case the document seems to be not correctly interpreted, return \"RESCAN DOCUMENT\".\n\n--- document ocr content (first 2000 chars) ---\n\n{DOCUMENT_CONTENT}"
        }
      ]
    }
  ]
}
```

### Text Extraction from Images
```http
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "google/gemma-3-27b-it",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "You job is to extract text from the images I provide you. Extract every bit of the text in the image.\nDon't say anything just do your job. Text should be same as in the images.\n\nIf the pages do not contain any text, just return \"Blank page\" or \"Image with no text\" or \"Image of <image description>\".\n\nThings to avoid:\n- Don't miss anything to extract from the images\n\nThings to include:\n- Include everything, even anything inside [], (), {} or anything.\n- Include any repetitive things like \"...\" or anything\n- If you think there is any mistake in image just include it too"
        },
        {
          "type": "image_url",
          "image_url": {
            "url": "data:{mime_type};base64,{base64_image}"
          }
        }
      ]
    }
  ]
}
```

## Response Patterns

### Paperless NGX Document List
```json
{
  "count": 1250,
  "results": [
    {
      "id": 123,
      "title": "Document Title",
      "content": "OCR content...",
      "created_date": "2024-05-15T10:30:00Z"
    }
  ]
}
```

### LLM Chat Completion
```json
{
  "choices": [
    {
      "message": {
        "content": "Response content"
      }
    }
  ]
}
```

### LLM Models List
```json
{
  "data": [
    {
      "id": "model-name"
    }
  ]
}
```

## Configuration

- **Paperless NGX**: `http://YOUR_PAPERLESS_HOST:8000/api/`
- **LLM API**: `http://YOUR_LLM_HOST:9876/v1/`
- **Image Formats**: JPEG, PNG (base64 encoded)
- **Content Truncation**: 2000 characters for title generation
