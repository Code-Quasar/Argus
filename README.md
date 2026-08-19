# Argus

Argus is a multi-provider LLM gateway written in Go. It exposes one OpenAI-compatible chat completions endpoint and routes requests to configured provider models.

Argus currently supports:

- OpenAI-compatible providers such as Groq, Mistral, Cerebras, OpenRouter, and Zhipu
- Gemini through its native `generateContent` API
- Priority-based model fallback
- Provider and model pause controls
- Round-robin API-key selection per provider
- Encrypted local storage for API keys and settings
- Foreground and background server modes

## Requirements

- Go 1.25 or newer
- API key for at least one supported provider

## Build

Clone the repository and build the binary:

```bash
git clone <repository-url>
cd Argus
go build -o Argus .
```

Run directly during development:

```bash
go run . --help
```

To make the binary available everywhere on Linux:

```bash
mkdir -p ~/.local/bin
install -m 755 ./Argus ~/.local/bin/Argus
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

Verify the installation:

```bash
Argus --help
```

## First Run And Security

On first use, Argus asks you to create a password. The password protects API keys, priorities, pause settings, and statistics.

The encrypted store is kept at:

```text
~/.argus/store.json
```

The directory is created with user-only permissions, and the store is encrypted using AES-GCM with an Argon2id-derived key.

Argus does not provide password recovery. To delete the password and all stored data:

```bash
Argus reset
```

You must type `DELETE` to confirm. This operation permanently removes the encrypted store.

## Add API Keys

API keys are stored encrypted locally. The key is passed as an argument, so prefer an environment variable to avoid placing the raw key directly in shell history:

```bash
export GEMINI_API_KEY="your-gemini-key"
Argus keys add gemini "$GEMINI_API_KEY"
```

OpenAI-compatible providers use the same command pattern:

```bash
export GROQ_API_KEY="your-groq-key"
Argus keys add groq "$GROQ_API_KEY"

export MISTRAL_API_KEY="your-mistral-key"
Argus keys add mistral "$MISTRAL_API_KEY"
```

Add an optional label:

```bash
Argus keys add gemini "$GEMINI_API_KEY" --label "personal account"
```

List connected keys. Keys are masked in output:

```bash
Argus keys list
Argus keys list --provider gemini
```

Supported provider names currently include:

```text
gemini
groq
mistral
cerebras
openrouter
zhipu
```

## Model Routing

Clients always send the model name `argus`:

```json
{
  "model": "argus",
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    }
  ]
}
```

Argus internally selects a model from its provider catalog. It sorts models by priority, skips paused models, tries the selected provider endpoint, and falls back to the next model when the upstream request fails.

A request returns an error only after all available model routes fail.

Set a model priority with a lower number meaning higher priority:

```bash
Argus priority gemini gemini-2.5-flash 0
Argus priority groq llama-3.1-8b-instant 1
```

Pause and resume providers or individual models:

```bash
Argus pause gemini
Argus pause gemini --model gemini-2.5-flash
Argus resume gemini
Argus resume gemini --model gemini-2.5-flash
```

## Start The Gateway

Start in the foreground:

```bash
Argus serve
```

The server listens on port `8080` by default. Use another port with:

```bash
Argus serve --port 9090
```

Start in the background:

```bash
Argus serve --background
```

Background logs are written to:

```text
~/.argus/argus.log
```

Check or stop the background server:

```bash
Argus status
Argus stop
```

## Make A Chat Request

The public API is OpenAI-compatible:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "argus",
    "messages": [
      {"role": "user", "content": "Explain what Argus does in one sentence."}
    ]
  }'
```

Health check:

```bash
curl http://localhost:8080/healthz
```

The response uses the unified OpenAI-compatible structure, including:

- `id`
- `model`
- `choices`
- `usage`

The returned `model` is normalized to `argus`, even though an internal provider model handled the request.

## Usage Statistics

Show request counts for today:

```bash
Argus stats --period day
```

These commands display counters stored by Argus. The current gateway does not yet persist a counter for every completed upstream request.

Show request counts for the current month:

```bash
Argus stats --period month
```

Filter statistics by provider:

```bash
Argus stats --period day --provider gemini
```

## Development

Run the test suite:

```bash
go test ./...
```

Run static analysis:

```bash
go vet ./...
```

Format the source:

```bash
gofmt -w $(find . -name '*.go' -type f -not -path './.git/*')
```

## Architecture

```text
OpenAI-compatible client
          |
          v
Chi HTTP router
          |
          v
Argus model router
          |
    +-----+-----+
    |           |
OpenAI-style  Gemini adapter
providers     native schema
    |           |
    +-----+-----+
          |
          v
Provider API endpoints
```

The main packages are:

- `api`: HTTP routing, provider requests, fallback logic, and response normalization
- `cmd`: CLI commands, encrypted storage, password handling, and process lifecycle
- `capacity`: provider endpoints and model catalog
- `convert`: native provider request and response conversion
- `schema`: unified and provider-specific data structures

## License

License information will be added when the project license is selected.
