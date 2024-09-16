# AI Command Translator CLI

This Go CLI tool allows you to translate natural language text commands into executable CLI commands. It supports multiple AI providers, including Cloudflare Workers AI LLMs, Anthropic, and OpenAI.

## Installation

First ensure you have Go installed. Then run:

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

## Configuration

To configure the AI provider and other settings, use:

```
ai config
```

This will open a configuration menu where you can set up your preferred LLM.
