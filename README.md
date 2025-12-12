# Azure AI CLI

> **⚠️ DEPRECATED**: This project has been moved to [https://github.com/quocvuong92/ai-cli](https://github.com/quocvuong92/ai-cli). Please use the new repository for the latest updates and features.

A modern command-line interface for Azure OpenAI with AI-powered command execution, web search, and interactive chat.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)

## ✨ Features

- 🤖 **AI Command Execution** - Let AI run terminal commands with intelligent permission system
- 🔍 **Web Search** - Integrated search via Tavily, Linkup, or Brave Search
- 💬 **Interactive Mode** - Chat sessions with history and auto-completing commands
- 🎨 **Markdown Rendering** - Beautiful syntax highlighting
- ⚡ **Streaming Output** - Real-time responses
- 🔄 **API Key Rotation** - Automatic rotation for free tier limits

## 🚀 Quick Start

### Installation

```bash
git clone https://github.com/yourusername/azure-ai-cli
cd azure-ai-cli
go build -o azure-ai
```

### Configuration

```bash
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_API_KEY="your-api-key"
export AZURE_OPENAI_MODELS="gpt-4o,gpt-4"  # Optional: comma-separated
```

### Basic Usage

```bash
# Simple query
azure-ai "What is Kubernetes?"

# Interactive mode with all features
azure-ai -sri

# Web search with citations
azure-ai -wc "Latest AI news"
```

## 💡 Command Execution

The AI can safely execute commands on your behalf:

```bash
$ azure-ai -i
> Show me what's in this directory
🔧 Executing: ls -la
[output displayed]

> Create a hello world in Python
⚠️  Command: echo 'print("Hello World")' > hello.py
Allow? [y/n/a]: y
✅ File created
```

**Safety Levels:**
- 🟢 **Safe** - Auto-approved (ls, cat, git status)
- 🟡 **Moderate** - Asks permission (git commit, npm install)
- 🔴 **Dangerous** - Blocked by default (rm -rf, sudo)

## 🌐 Web Search

Add real-time web data to your queries:

```bash
# Enable web search
export TAVILY_API_KEYS="your-key"

# Query with automatic search
azure-ai -w "What's new in Go 1.24?"
```

**Supported Providers:**
- [Tavily](https://tavily.com) - Full-featured search
- [Linkup](https://linkup.so) - Alternative provider
- [Brave Search](https://brave.com/search/api/) - Privacy-focused (2K free queries/month)

## 🎮 Interactive Mode

```bash
azure-ai -i  # Start interactive session
```

**Slash Commands:**
- `/web on/off` - Toggle web search
- `/model <name>` - Switch models
- `/clear` - Clear history
- `/allow-dangerous` - Enable risky commands
- Type `/` for auto-complete

## 📚 Common Examples

```bash
# Code generation with streaming
azure-ai -s "Write a REST API in Go"

# Research with sources
azure-ai -wc "Best practices for microservices"

# Quick terminal help
azure-ai "How to find large files on macOS?"

# Interactive coding session
azure-ai -sri
```

## ⚙️ Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `AZURE_OPENAI_ENDPOINT` | ✅ | Your Azure OpenAI endpoint |
| `AZURE_OPENAI_API_KEY` | ✅ | API key |
| `AZURE_OPENAI_MODELS` | ❌ | Available models (default: gpt-5.1-chat) |
| `TAVILY_API_KEYS` | ❌ | Tavily keys (comma-separated) |
| `LINKUP_API_KEYS` | ❌ | Linkup keys (comma-separated) |
| `BRAVE_API_KEYS` | ❌ | Brave Search keys |
| `WEB_SEARCH_PROVIDER` | ❌ | Default provider (tavily/linkup/brave) |

### Flags

```
-i, --interactive    Interactive chat mode
-s, --stream        Stream responses
-r, --render        Render markdown
-w, --web          Enable web search
-c, --citations    Show sources
-m, --model        Select model
-u, --usage        Show token usage
-v, --verbose      Debug mode
```

## 🔒 Security

- ✅ Pattern-based command classification
- ✅ User confirmation for write operations
- ✅ Dangerous commands blocked by default
- ✅ 30-second execution timeout
- ✅ Session-based allowlist

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🔗 Resources

- [Azure OpenAI Documentation](https://learn.microsoft.com/en-us/azure/ai-services/openai/)
- [Implementation Guide](IMPLEMENTATION_GUIDE.md) - Add this feature to your own projects
- [Web Search Providers](https://github.com/yourusername/azure-ai-cli/wiki/Web-Search-Providers)

---

**Built with ❤️ using Go and Azure OpenAI**
