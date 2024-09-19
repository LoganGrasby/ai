package main

import (
	"fmt"
	"path/filepath"
	"runtime"

	usearch "github.com/unum-cloud/usearch/golang"
)

const (
	configFileName             = "ai-config.yaml"
	cacheFileName              = "ai_cache.json"
	indexFileName              = "index.usearch"
	maxRetries                 = 3
	LLMRequest     RequestType = iota
	EmbeddingsRequest
)

var (
	storeDir, currentDir = getDir()
	cacheFile            = filepath.Join(storeDir, cacheFileName)
	indexFile            = filepath.Join(storeDir, indexFileName)
	configFile           = filepath.Join(storeDir, configFileName)
	osInfo               = runtime.GOOS
	maxTokens            = 1000
	temperature          = 0.5
	index                *usearch.Index
	keyToUint64          map[string]uint64
	uint64ToKey          map[uint64]string
	k                    = 1
	defaultMaxDistance   = 0.2
	vectorSize           = 1536
)

var systemPrompt = fmt.Sprintf("Translate the following text command to a CLI command for %s. The current working directory is %s. Output the command within XML tags like this: <command>CLI command</command>", osInfo, currentDir)
