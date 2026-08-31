# KakaoBot

KakaoBot is a modular Go library and CLI tool for interacting with Kakao REST APIs, sending KakaoTalk messages (memo, friends, templates), managing user sessions and CDN storage, running inbound webhook relay servers, and hosting Kakao i OpenBuilder chatbot skill webhook servers.

## Architecture

The codebase adheres strictly to SOLID and Clean Architecture principles:
- **Dependency Inversion (DIP)**: High-level use cases and callers depend entirely on domain interfaces (`kakao.Client`, `kakao.AuthService`, `kakao.UserService`, `kakao.MemoService`, `kakao.FriendsService`, `kakao.StorageService`, `kakao.TokenStore`).
- **Interface Segregation (ISP)**: Services are broken down into focused interfaces instead of a single bloated monolith.
- **Single Responsibility (SRP)**: Each service implementation (`auth_service.go`, `memo_service.go`, `friends_service.go`, `user_service.go`, `storage_service.go`) handles only its respective API domain.
- **Functional Collections**: Collection transformations and filters use `github.com/samber/lo`.
- **CLI Standard**: CLI commands are structured using Cobra and struct-tag binding via `github.com/dh-kam/refutils/flagsbinder`.

```
pkg/kakao/
├── interfaces.go       # Core domain interfaces
├── types.go            # Data models and API types
├── errors.go           # Error types
├── template.go         # Message template builders (Text, Feed, List, Commerce, Location)
├── client.go           # Composite Client interface implementation
├── auth_service.go     # OAuth2, token exchange, token refresh, logout, unlink
├── user_service.go     # User profile and shipping address retrieval
├── memo_service.go     # Send messages to oneself (/v2/api/talk/memo/*)
├── friends_service.go  # Friend list discovery and friend messaging
├── storage_service.go  # Kakao CDN image upload, scrape, and deletion
└── token_store.go      # Thread-safe file and memory token stores

pkg/openbuilder/
├── types.go            # Kakao i OpenBuilder skill payload and response types
└── builder.go          # Fluent response builder (SimpleText, BasicCard, Carousel, QuickReplies)
```

## Installation and Build

The project uses an OS-ARCH-VARIANT build matrix in the `Makefile`.

```bash
# Build debug binary for current architecture
make linux-amd64-debug

# Build statically linked standalone release binary
make linux-amd64-release

# Run unit tests with race detection and linter
make test
make lint

# Clean build artifacts
make clean
```

Artifacts are produced in `build/<os>-<arch>/<variant>/kakaobot`.

## Docker

The `Dockerfile` builds a statically linked release binary (with version
metadata injected from git) and packages it into a minimal Alpine image that
runs as a non-root user and exposes the webhook server on port 8080 with a
`HEALTHCHECK` on `/healthz`. `tools/manage.sh` automates builds and pushes to
a private repository in the Oracle Cloud Infrastructure Registry (OCIR).

Registry configuration (export or place in `.env`):

```dotenv
OCI_REGION=ap-seoul-1
OCI_TENANCY_NAMESPACE=your-tenancy-namespace
OCI_USERNAME=oracleidentitycloudservice/you@example.com
OCI_AUTH_TOKEN=your-oci-auth-token
IMAGE_REPOSITORY=kakao-bot
```

```bash
# Authenticate to OCIR (auth token, not the account password)
./tools/manage.sh login

# Build the image locally; tag defaults to `git describe --tags --always`
./tools/manage.sh build

# Build and push :<tag> and :latest
./tools/manage.sh push

# Multi-arch manifest push
PLATFORMS=linux/amd64,linux/arm64 ./tools/manage.sh push
```

Run the webhook relay server:

```bash
docker run -d --name kakao-bot \
  -p 8080:8080 \
  -e KAKAO_REST_API_KEY=your_rest_api_key \
  -v kakao-bot-config:/home/kakaobot/.config/kakao-bot \
  ap-seoul-1.ocir.io/your-tenancy-namespace/kakao-bot:latest
```

Kakao credentials (`KAKAO_REST_API_KEY`, `KAKAO_CLIENT_SECRET`) are read from
the environment. OAuth tokens are persisted under
`/home/kakaobot/.config/kakao-bot`, so mount a volume there to keep sessions
across restarts. Additional `serve` flags such as `--secret-token` can be
appended to the `docker run` arguments.

## Configuration

Credentials can be provided via environment variables or a local `.env` file:

```dotenv
KAKAO_REST_API_KEY=your_rest_api_key
KAKAO_CLIENT_SECRET=your_client_secret
KAKAO_REDIRECT_URI=http://localhost:8080/callback
KAKAO_TOKEN_PATH=~/.config/kakao-bot/tokens.json
```

## CLI Usage

### Authentication (`auth`)

```bash
# Verify REST API Key configuration
kakaobot auth check

# OAuth 2.0 login with automatic local callback listener
kakaobot auth login

# Check authentication status and token expiry
kakaobot auth status

# Force refresh access token
kakaobot auth refresh

# Logout and clear local tokens
kakaobot auth logout

# Unlink app connection
kakaobot auth unlink
```

### Messaging (`send`)

```bash
# Send text message to oneself
kakaobot send me "Deployment completed successfully"

# Send text with URL and custom button
kakaobot send me "New release available" --url "https://github.com" --button "View Release"

# Pipe text from stdin
uptime | kakaobot send me

# Send rich feed card
kakaobot send me \
  --title "System Alert" \
  --description "High CPU usage detected: 92%" \
  --image-url "https://via.placeholder.com/600x400.png" \
  --url "https://dashboard.example.com" \
  --button "Open Dashboard"

# Send message to friends by UUID
kakaobot send friend -R "<RECEIVER_UUID>" "Hello friend"
```

### User Profile (`user`)

```bash
# Get authenticated user profile
kakaobot user me

# Query registered shipping addresses
kakaobot user address
```

### CDN Storage (`storage`)

```bash
# Upload image to Kakao CDN for use in messages
kakaobot storage upload ./alert-chart.png

# Delete uploaded image from Kakao CDN
kakaobot storage delete "https://k.kakaocdn.net/..."
```

### Webhook Relay Server (`serve`)

Starts an HTTP server that relays incoming webhook alerts (e.g. Grafana, Alertmanager, GitHub) to KakaoTalk:

```bash
kakaobot serve --listen ":8080"
```

Send webhook:
```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -d '{"text": "Server alert from Prometheus"}'
```

### Chatbot Skill Server with AI (`skill`)

Starts a Kakao i OpenBuilder skill webhook server for channel `@0xc0de1ab` with pluggable AI LLM integration (OpenAI, Gemini, Claude, Ollama, Groq, DeepSeek, Mock):

```bash
# Run with OpenAI (or any OpenAI-compatible provider)
kakaobot skill serve --listen ":8080" \
  --ai-provider "openai" \
  --ai-api-key "sk-..." \
  --ai-model "gpt-4o-mini"

# Run with Google Gemini
kakaobot skill serve --listen ":8080" \
  --ai-provider "gemini" \
  --ai-api-key "AIza..." \
  --ai-model "gemini-1.5-flash"

# Run with local Ollama
kakaobot skill serve --listen ":8080" \
  --ai-provider "ollama" \
  --ai-base-url "http://localhost:11434/v1" \
  --ai-model "llama3"
```

## Go Library Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
)

func main() {
    client := kakao.NewClient(kakao.ClientConfig{
        ClientID:     "your-rest-api-key",
        ClientSecret: "your-client-secret",
    })

    ctx := context.Background()

    // Send memo text
    err := client.Memo().SendText(ctx, kakao.TextMessageRequest{
        Text:        "Hello from Go client!",
        WebURL:      "https://github.com",
        ButtonTitle: "Visit",
    })
    if err != nil {
        log.Fatalf("send memo: %v", err)
    }

    // Get user profile
    profile, err := client.User().GetMe(ctx)
    if err != nil {
        log.Fatalf("get profile: %v", err)
    }
    fmt.Printf("User ID: %d\n", profile.ID)
}
```
