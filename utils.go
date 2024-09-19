package main

import (
	"log"
	"os"
)

func getDir() (string, string) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        log.Fatalf("Error getting user home directory: %v", err)
    }

    currentDir, err := os.Getwd()
    if err != nil {
        log.Fatalf("Error getting current directory: %v", err)
    }

    return homeDir, currentDir
}
