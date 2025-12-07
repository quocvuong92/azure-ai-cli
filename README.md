# Azure AI CLI

A command-line client for Azure OpenAI API with optional web search powered by Tavily.

## Features

- Chat with Azure OpenAI models (GPT-5.1, GPT-4o, etc.)
- Web search integration via Tavily API
- Streaming output support
- Markdown rendering with syntax highlighting
- Multiple Tavily API keys with automatic rotation (for free tier usage)
- Token usage statistics

## Installation

### From Source

```bash
# Clone and build
cd azure-ai-cli
make build

# Install to ~/go/bin
make install
```

### Binary

```bash
# After building
cp bin/azure-ai /usr/local/bin/
```

## Configuration

Set the following environment variables:

```bash
# Required - Azure OpenAI
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_API_KEY="your-api-key"

# Optional - Available models (comma-separated, first one is default)
export AZURE_OPENAI_MODELS="gpt-5.1-chat,gpt-5.1,gpt-4o"

# Optional - Tavily for web search (enables --web flag)
# Supports multiple keys for free tier rotation
export TAVILY_API_KEYS="tvly-key1,tvly-key2,tvly-key3"
```

**Note:** If `AZURE_OPENAI_MODELS` is not set, the default model is `gpt-5.1-chat`.

## Usage

### Basic Query

```bash
azure-ai "What is Kubernetes?"
```

### Select Model

```bash
azure-ai -m gpt-5.1 "Explain Docker containers"
```

### Streaming Output

```bash
azure-ai -s "Write a hello world in Go"
```

### Rendered Markdown

```bash
azure-ai -r "Show me a code example for HTTP server"
```

### Streaming + Rendered

```bash
azure-ai -sr "Explain microservices architecture"
```

### Web Search

Search the web first, then use the results to generate an answer:

```bash
azure-ai --web "What is the latest version of Kubernetes?"
azure-ai -w "Latest news on AI"

# With citations/sources
azure-ai -wc "What is the latest version of Kubernetes?"
```

### Combined Flags

```bash
azure-ai -srw "What happened in tech news today?"
```

### Show Token Usage

```bash
azure-ai -u "Hello world"
```

### List Available Models

```bash
azure-ai --list-models
```

### Verbose/Debug Mode

```bash
azure-ai -v "Test query"
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--model` | `-m` | Model/deployment name |
| `--stream` | `-s` | Stream output in real-time |
| `--render` | `-r` | Render markdown with colors |
| `--web` | `-w` | Search web first using Tavily |
| `--citations` | `-c` | Show citations/sources from web search |
| `--interactive` | `-i` | Start interactive chat mode |
| `--usage` | `-u` | Show token usage statistics |
| `--verbose` | `-v` | Enable debug mode |
| `--list-models` | | List available models |
| `--help` | `-h` | Show help |

## Web Search

When using `--web` flag:

1. Tavily searches the web for relevant information
2. Search results are sent to Azure OpenAI as context
3. The model generates an answer based on the search results
4. Sources are displayed at the end with citations

The model will cite sources using `[1]`, `[2]`, etc. in its response.

### Tavily API Key Rotation

If you have multiple Tavily free accounts, you can provide multiple API keys:

```bash
export TAVILY_API_KEYS="tvly-key1,tvly-key2,tvly-key3"
```

When one key hits rate limits (429) or is exhausted, the CLI automatically switches to the next key.

## Interactive Mode

Start an interactive chat session with conversation history:

```bash
azure-ai -i
azure-ai --interactive

# With streaming and rendering
azure-ai -sri

# With auto web search for every message
azure-ai -iwsr
```

### Features

- **Conversation History**: Messages are kept in memory, allowing follow-up questions
- **Slash Commands**: Control the session with built-in commands
- **Web Search**: Use `-w` flag for auto web search, or `/web` command for one-off searches

### Slash Commands

| Command | Description |
|---------|-------------|
| `/exit`, `/quit`, `/q` | Exit interactive mode |
| `/clear`, `/c` | Clear conversation history |
| `/help`, `/h` | Show available commands |
| `/web <query>` | Search the web and ask about results |
| `/web on` | Enable auto web search for all messages |
| `/web off` | Disable auto web search |
| `/web` | Show current web search status |
| `/model [name]` | Show or change the current model |

### Example Session

```
$ azure-ai -sri
Azure AI CLI - Interactive Mode
Model: gpt-5.1-chat
Type /help for commands, /exit to quit

> What is Kubernetes?
Kubernetes is an open-source container orchestration platform...

> How does it compare to Docker Swarm?
Building on what I explained about Kubernetes, here's how it compares...

> /web on
Web search enabled for all messages.

> What is the latest Kubernetes version?
[Searches web automatically]
According to recent information, Kubernetes 1.32 was released...

> /web off
Web search disabled.

> /clear
Conversation cleared.

> /exit
Goodbye!
```

## Examples

```bash
# Simple question
azure-ai "What is Go programming language?"

# Code generation with streaming
azure-ai -s "Write a function to reverse a string in Python"

# Research with web search
azure-ai -w "What are the new features in Go 1.24?"

# Full featured query
azure-ai -srw -m gpt-5.1-chat "Latest developments in AI agents"

# Interactive chat with history
azure-ai -sri
```

## License

MIT
