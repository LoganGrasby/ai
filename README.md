# AI Command Translator CLI

This Go CLI tool allows you to translate natural language text commands into executable CLI commands. It supports multiple AI providers, including Cloudflare Workers AI LLMs, Anthropic, and OpenAI.

Local LLMs are also supported with [Ollama](https://ollama.com/)


## Installation

First ensure you have [Go](https://go.dev/) installed.

Ensure your PATH and GOPATH are set correctly in your shell profile (e.g. `~/.bashrc`, `~/.zshrc`, etc.):
```
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-L/usr/local/lib"
export DYLD_LIBRARY_PATH=/usr/local/lib:$DYLD_LIBRARY_PATH
```

Install:
```
git clone https://github.com/LoganGrasby/ai
cd ai
make install
```

This will:
1. Add the necessary environment variables to your shell profile (if they don't already exist)
2. Install [Usearch](https://github.com/unum-cloud/usearch)
3. Install the `ai` CLI tool to your `$GOPATH/bin` directory.

If you have issues installing usearch see the installation instructions [here](https://github.com/unum-cloud/usearch/blob/main/golang/README.md)


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

## Semantic Cache

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
