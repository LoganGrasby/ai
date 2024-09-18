package main

import (
	"fmt"
	"os"
)

func getUserHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Errorf("Error getting user home directory: %v", err)
		os.Exit(1)
	}
	return homeDir
}
