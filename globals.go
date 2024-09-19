package main

import (
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	configFileName             = "ai-cli-config.yaml"
	cacheFileName              = "ai_cache.json"
	maxRetries                 = 3
	LLMRequest     RequestType = iota
	EmbeddingsRequest
)

var (
	homeDir, currentDir = getDir()
	cacheFile           = filepath.Join(homeDir, cacheFileName)
	osInfo              = runtime.GOOS
	maxTokens           = 1000
	temperature         = 0.5
)

var systemPrompt = fmt.Sprintf("Translate the following text command to a CLI command for %s. The current working directory is %s. Output the command within XML tags like this: <command>CLI command</command>", osInfo, currentDir)
