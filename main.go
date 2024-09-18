package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var cache map[string]string

func main() {
	loadConfig()
	loadCache()

	if len(os.Args) >= 2 && os.Args[1] == "config" {
		configMenu()
		return
	}

	var fullCommand string

	if len(os.Args) > 1 {
		fullCommand = strings.Join(os.Args[1:], " ")
	} else {
		fmt.Print("Enter your command: ")
		reader := bufio.NewReader(os.Stdin)
		cmd, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			os.Exit(1)
		}
		fullCommand = strings.TrimSpace(cmd)
	}

	executeCommand(fullCommand)
}

func confirmExecution() bool {
	fmt.Print("Do you want to execute this command? (y/n): ")
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}
