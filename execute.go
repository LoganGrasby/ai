package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/viper"
)

func executeCommand(textCommand string) {
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	apiKey := getAPIKey(provider)
	if apiKey == "" {
		fmt.Printf("Error: API key not set for provider %s. Use 'ai config' or set the appropriate environment variable.\n", provider)
		os.Exit(1)
	}

	osInfo := runtime.GOOS
	prompt := fmt.Sprintf("Translate the following text command to a CLI command for %s. Output the command within XML tags like this: <command>CLI command</command>\n\nText command: %s", osInfo, textCommand)

	responseText, err := callAPI(provider, model, apiKey, prompt)
	if err != nil {
		fmt.Printf("Error calling %s API: %v\n", provider, err)
		os.Exit(1)
	}

	var cmd Command
	err = xml.Unmarshal([]byte(responseText), &cmd)
	if err != nil || cmd.Content == "" {
		fmt.Printf("Error parsing LLM response: %v\nRaw response: %s\n", err, responseText)
		os.Exit(1)
	}

	fmt.Println("Generated command:", cmd.Content)

	if viper.GetBool("require_confirmation") {
		if !confirmExecution() {
			fmt.Println("Command execution cancelled.")
			return
		}
	}

	fmt.Println("Executing command...")

	execCmd := exec.Command("sh", "-c", cmd.Content)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	err = execCmd.Run()
	if err != nil {
		fmt.Printf("Error executing command: %v\n", err)
		os.Exit(1)
	}
}
