package main

import (
	"go/build"
	"log"
	"os"
	"path/filepath"
)

func getDir() (string, string) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}

	if gopath == "" {
		log.Fatal("GOPATH is not set and couldn't be determined")
	}

	storeDir := filepath.Join(gopath, "src", "github.com", "LoganGrasby", "ai")

	err := os.MkdirAll(storeDir, 0755)
	if err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	return storeDir, currentDir
}
