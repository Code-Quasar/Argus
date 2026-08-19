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

API keys are entered through hidden terminal input and are stored encrypted locally:

```bash
Argus keys add gemini
```

Argus prompts for `Enter API key:`. The characters are not displayed. The same command works for all supported providers:

```bash
Argus keys add groq
Argus keys add mistral
Argus keys add cerebras
Argus keys add openrouter
Argus keys add zhipu
```

Add an optional label:

```bash
Argus keys add gemini --label "personal account"
```

Configure every supported provider in one wizard. Press `Enter` without entering a key to skip a provider:

```bash
Argus full-setup
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

The request must use `model: "argus"`. The response reports the actual internal model that successfully answered, for example `gemini-2.5-flash` or `llama-3.1-8b-instant`.

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

## Command Reference

Show help for all commands:

```bash
Argus --help
Argus <command> --help
```

Available commands:

| Command | Description |
| --- | --- |
| `Argus full-setup` | Prompt for keys for every supported provider. |
| `Argus keys add <provider>` | Securely prompt for and save one provider key. |
| `Argus keys list` | List masked keys. |
| `Argus keys list --provider <provider>` | List keys for one provider. |
| `Argus priority <provider> <model> <number>` | Set model priority. Lower numbers are tried first. |
| `Argus pause <provider>` | Pause all models for a provider. |
| `Argus pause <provider> --model <model>` | Pause one model. |
| `Argus resume <provider>` | Resume all models for a provider. |
| `Argus resume <provider> --model <model>` | Resume one model. |
| `Argus stats --period day` | Show current-day stored counters. |
| `Argus stats --period month` | Show current-month stored counters. |
| `Argus serve` | Run the gateway in the foreground. |
| `Argus serve --port <port>` | Run on a custom port. |
| `Argus serve --background` | Run as a detached background process. |
| `Argus status` | Check the background server status. |
| `Argus stop` | Stop the background server. |
| `Argus reset` | Delete the encrypted store and all saved settings. |

`Argus reset` is destructive and requires typing `DELETE`. It is the recovery path when the encryption password is forgotten, but all saved API keys and settings will be lost.

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
