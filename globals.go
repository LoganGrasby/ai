package main

import (
	"fmt"
	"os"
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
var currentDir, _ = os.Getwd()

var systemPrompt = fmt.Sprintf("Translate the following text command to a CLI command for %s. The current working directory is %s. Output the command within XML tags like this: <command>CLI command</command>", osInfo, currentDir)
