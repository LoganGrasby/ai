package main

import (
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	configFileName = "ai-cli-config.yaml"
	cacheFileName  = "ai_cache.json"
	maxRetries     = 3
)

var homeDir = getUserHomeDir()
var cacheFile = filepath.Join(homeDir, cacheFileName)
var osInfo = runtime.GOOS

var systemPrompt = fmt.Sprintf("Translate the following text command to a CLI command for %s. Output the command within XML tags like this: <command>CLI command</command>", osInfo)
