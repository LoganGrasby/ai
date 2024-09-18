# AI Command Translator CLI

This Go CLI tool allows you to translate natural language text commands into executable CLI commands. It supports multiple AI providers, including Cloudflare Workers AI LLMs, Anthropic, and OpenAI.

Local LLMs are also supported with [Ollama](https://ollama.com/)


## Installation

First ensure you have [Go](https://go.dev/) installed. Then run:
```
go install github.com/LoganGrasby/ai@latest
```

Ensure your PATH environment variable is configured correctly:

```
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

## Usage

You can use the CLI tool in several ways:

1. Directly with a command:
   ```
   ai list files in this directory
   ```

2. With quotes for commands containing special characters/punctuation:
   ```
   ai "How much memory do I have available?"
   ```

3. Interactive mode:
   ```
   ai
   > Enter your command: List files in this directory
   ```

Successful commands are cached. Errors are sent back to the model to retry:

```
% ai check my available system ram
Generated command: free -m
Executing command...
Error executing command: sh: free: command not found
Generated command: top -l 1 | grep PhysMem
Executing command...
PhysMem: 14G used (2169M wired, 607M compressor), 1192M unused.
```
The successful command is cached:
```
% ai check my available system ram
Cached command: top -l 1 | grep PhysMem
Executing command...
PhysMem: 14G used (2149M wired, 607M compressor), 1187M unused.
```

## Configuration

To configure the AI provider and other settings, use:

```
ai config
```

This will open a configuration menu where you can set up your preferred LLM.
