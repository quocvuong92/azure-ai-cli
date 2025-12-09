# Azure AI CLI

A command-line client for Azure OpenAI API with optional web search powered by Tavily, Linkup, or Brave.

## Features

- Chat with Azure OpenAI models (GPT-5.1, GPT-4o, etc.)
- **AI-powered command execution** with intelligent permission system
- Web search integration via Tavily, Linkup, or Brave Search API
- Interactive mode with auto-completing suggestions
- Streaming output support
- Markdown rendering with syntax highlighting
- Multiple API keys with automatic rotation (for free tier usage)
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

# Optional - Web search providers (enables --web flag)
# Tavily - supports multiple keys for free tier rotation
export TAVILY_API_KEYS="tvly-key1,tvly-key2,tvly-key3"

# Linkup - alternative web search provider
export LINKUP_API_KEYS="linkup-key1,linkup-key2"

# Brave Search - alternative web search provider
export BRAVE_API_KEYS="brave-key1,brave-key2"

# Optional - Select default web search provider (tavily, linkup, or brave)
# If not set, auto-detects based on available keys (prefers Tavily)
export WEB_SEARCH_PROVIDER="tavily"
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

# Use specific provider
azure-ai -w --provider linkup "Latest AI news"
azure-ai -w -p brave "What happened today?"
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
| `--web` | `-w` | Search web first using configured provider |
| `--provider` | `-p` | Web search provider: tavily, linkup, or brave |
| `--citations` | `-c` | Show citations/sources from web search |
| `--interactive` | `-i` | Start interactive chat mode |
| `--usage` | `-u` | Show token usage statistics |
| `--verbose` | `-v` | Enable debug mode |
| `--list-models` | | List available models |
| `--help` | `-h` | Show help |

## Web Search

When using `--web` flag:

1. The configured provider (Tavily, Linkup, or Brave) searches the web for relevant information
2. Search results are sent to Azure OpenAI as context
3. The model generates an answer based on the search results
4. Sources are displayed at the end with citations

The model will cite sources using `[1]`, `[2]`, etc. in its response.

### Providers

**Tavily** (default): Full-featured web search API with good results quality.
- Get API keys at: https://tavily.com

**Linkup**: Alternative web search provider.
- Get API keys at: https://linkup.so

**Brave Search**: Privacy-focused web search with generous free tier (2,000 queries/month).
- Get API keys at: https://brave.com/search/api/

### API Key Rotation

All providers support multiple API keys for free tier rotation:

```bash
export TAVILY_API_KEYS="tvly-key1,tvly-key2,tvly-key3"
export LINKUP_API_KEYS="linkup-key1,linkup-key2"
export BRAVE_API_KEYS="brave-key1,brave-key2"
```

When one key hits rate limits (429) or is exhausted, the CLI automatically switches to the next key.

## Command Execution

The AI can execute shell commands to help you with tasks. This feature uses Azure OpenAI's function calling with an intelligent permission system.

### How It Works

When you ask the AI to perform a task that requires running commands, it will:

1. **Decide** if a command is needed
2. **Check permissions** based on command risk level
3. **Ask for confirmation** if needed
4. **Execute** the command
5. **Return results** and explain what happened

### Permission System

Commands are classified into three risk levels:

**🟢 Safe (Auto-Execute)**
- Read-only commands that can't harm your system
- Examples: `ls`, `cat`, `pwd`, `git status`, `npm list`
- No confirmation needed - executes immediately

**🟡 Needs Confirmation**
- Commands that modify your system
- Examples: `git commit`, `npm install`, `mkdir`, `rm`
- AI asks: `Allow? [y]es / [n]o / [a]lways`
- Choose "always" to trust the command for this session

**🔴 Dangerous (Blocked)**
- Potentially destructive commands
- Examples: `rm -rf /`, `sudo`, `dd`, pipe-to-shell
- Blocked by default unless you run `/allow-dangerous`

### Examples

```bash
$ azure-ai -i
> What files are in this directory?
🔧 Executing: ls -la
[shows directory listing]
The directory contains...

> Create a file called test.txt with "Hello World"
⚠️  Command Execution Request
Command:  echo "Hello World" > test.txt
Reason:   User requested to create a file with content

Allow? [y]es / [n]o / [a]lways: y
🔧 Executing: echo "Hello World" > test.txt
I've created test.txt with the content "Hello World".

> Show me the git status
🔧 Executing: git status
[shows git status output]
You're on the main branch with no uncommitted changes.
```

### Safety Features

✅ Auto-approve safe reads - No friction for browsing  
✅ Confirm writes - Always ask before modifying  
✅ Block dangerous commands - Protect your system  
✅ Allowlist support - Build trust over time with "always allow"  
✅ 30-second timeout - Commands won't hang forever  
✅ Exit code tracking - Proper error handling  

### Controls

- `/allow-dangerous` - Enable dangerous commands (still requires confirmation)
- `/show-permissions` - View current permission settings
- Answer `n` to deny a command
- Answer `a` for "always allow" to skip future confirmations for that specific command

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
| `/allow-dangerous` | Enable dangerous commands (with confirmation) |
| `/show-permissions` | Show command execution permissions |

**Note**: Type `/` to see auto-completing suggestions with descriptions. Use Tab or arrow keys to navigate.

### Example Session

```
$ azure-ai -sri
Azure AI CLI - Interactive Mode
Model: gpt-5.1-chat
Type /help for commands, Ctrl+C to quit, Tab for autocomplete

> What is Kubernetes?
Kubernetes is an open-source container orchestration platform...

> How does it compare to Docker Swarm?
Building on what I explained about Kubernetes, here's how it compares...

> /web on
Web search enabled (provider: tavily).

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
