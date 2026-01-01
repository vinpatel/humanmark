# HumanMark

**Open source AI content detection. One API. Zero dependencies.**

Detect AI-generated text, images, audio, and video—entirely offline, with no external API calls.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

---

## Why HumanMark?

| Feature | HumanMark | GPTZero, etc. |
|---------|-----------|---------------|
| Self-hosted | ✅ | ❌ |
| Works offline | ✅ | ❌ |
| Zero cost | ✅ | ❌ |
| Privacy-first | ✅ | ❌ |
| Open source | ✅ | ❌ |
| Multi-modal | ✅ Text, Image, Audio, Video | ⚠️ Mostly text-only |

---

## Quick Start

### Option 1: Go Install

```bash
go install github.com/humanmark/humanmark/cmd/api@latest
humanmark
```

### Option 2: Docker

```bash
docker run -p 8080:8080 humanmark/humanmark
```

### Option 3: Build from Source

```bash
git clone https://github.com/humanmark/humanmark.git
cd humanmark
go run cmd/api/main.go
```

Server runs at `http://localhost:8080`

---

## Usage

### Detect Text

```bash
curl -X POST http://localhost:8080/verify \
  -H "Content-Type: application/json" \
  -d '{"text": "Your content here"}'
```

### Response

```json
{
  "id": "abc123",
  "human": true,
  "confidence": 0.85,
  "content_type": "text"
}
```

### Detailed Analysis

```bash
curl -X POST "http://localhost:8080/verify?detailed=true" \
  -H "Content-Type: application/json" \
  -d '{"text": "Your content here"}'
```

```json
{
  "id": "abc123",
  "human": true,
  "confidence": 0.85,
  "content_type": "text",
  "details": {
    "ai_score": 0.15,
    "detectors": ["humanmark"]
  }
}
```

---

## API Reference

### `POST /verify`

Analyze content for AI generation.

**Request Body:**

| Field | Type | Description |
|-------|------|-------------|
| `text` | string | Text content to analyze |
| `url` | string | URL to fetch and analyze |
| `file` | file | File upload (multipart) |

*Provide one of: `text`, `url`, or `file`*

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `detailed` | bool | false | Include analysis details |

**Response:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique result ID |
| `human` | bool | `true` = human, `false` = AI |
| `confidence` | float | 0.0 to 1.0 |
| `content_type` | string | text, image, audio, video |
| `details` | object | Present if `?detailed=true` |

---

### `GET /verify/{id}`

Retrieve a previous result.

---

### `GET /health`

Health check endpoint.

```json
{
  "status": "healthy",
  "timestamp": "2025-01-01T00:00:00Z",
  "checks": {
    "database": true
  }
}
```

---

## How It Works

HumanMark uses statistical and forensic analysis—no ML models required.

### Text Detection

| Signal | Human | AI |
|--------|-------|-----|
| Sentence length variance | High | Low |
| Vocabulary richness | Varied | "Safe" words |
| Contractions | "don't", "I'm" | "do not", "I am" |
| Punctuation variety | !?;:— | Mostly periods |
| AI phrases | Rare | "As an AI...", "It's important to note..." |

### Image Detection

| Signal | Real Photo | AI Image |
|--------|------------|----------|
| EXIF metadata | Present | Missing/fake |
| Camera make | Apple, Canon, etc. | None |
| Sensor noise | Natural pattern | Too clean |
| Compression | Normal JPEG | Unusual artifacts |

### Audio Detection

| Signal | Real Audio | AI Audio |
|--------|------------|----------|
| Recording markers | Studio, device info | Missing |
| Encoder | ffmpeg, Audacity | ElevenLabs, etc. |
| Noise profile | Natural | Synthetic |

### Video Detection

| Signal | Real Video | AI Video |
|--------|------------|----------|
| Audio track | Present | Often missing |
| Encoder | Premiere, ffmpeg | Runway, Pika, Sora |
| Container structure | Standard | Unusual |

---

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `ENV` | development | Environment (development/production) |
| `LOG_LEVEL` | info | Logging level |
| `DATABASE_URL` | - | PostgreSQL connection (optional) |
| `REDIS_URL` | - | Redis connection (optional) |

---

## Development

### Run Tests

```bash
go test ./... -v
```

### Run with Hot Reload

```bash
go install github.com/cosmtrek/air@latest
air
```

### Build Binary

```bash
go build -o humanmark cmd/api/main.go
```

### Build Docker Image

```bash
docker build -t humanmark .
```

---

## Project Structure

```
humanmark/
├── cmd/api/              # Application entrypoint
│   └── main.go
├── internal/
│   ├── config/           # Configuration
│   ├── handler/          # HTTP handlers
│   ├── middleware/       # HTTP middleware
│   ├── repository/       # Data storage
│   └── service/          # Detection algorithms ⭐
│       ├── text_analyzer.go
│       ├── image_analyzer.go
│       ├── audio_analyzer.go
│       └── video_analyzer.go
├── pkg/logger/           # Structured logging
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

---

## Roadmap

- [x] Text detection
- [x] Image detection
- [x] Audio detection
- [x] Video detection
- [ ] ML model training pipeline
- [ ] Browser extension
- [ ] WordPress plugin
- [ ] Batch processing API
- [ ] Dashboard UI

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Good first issues:**
- Improve detection accuracy
- Add tests
- Documentation improvements
- New integrations

---

## Accuracy

Current accuracy is estimated at **60-75%** on general content.

We're actively working to improve this through:
1. Community feedback
2. Dataset collection
3. ML model training

Help us improve by [reporting false positives/negatives](https://github.com/humanmark/humanmark/issues/new).

---

## License

MIT License. See [LICENSE](LICENSE).

---

## Acknowledgments

Built by the open source community. Inspired by the need for transparent, privacy-respecting AI detection.

---

**Star ⭐ this repo if you find it useful!**
