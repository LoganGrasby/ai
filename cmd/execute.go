package cmd

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

type RequestType int

const (
	maxRetries             = 3
	LLMRequest RequestType = iota
	EmbeddingsRequest
)

func executeCommand(textCommand string) {
	cachedResponse, found, vector := getCachedResponse(textCommand)
	if found {
		fmt.Println("Cached command:", cachedResponse)
		err := executeCLICommand(cachedResponse)
		if err != nil {
			fmt.Println("Error executing cached command:", err)
		}
		return
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	apiKey := getAPIKey(provider)
	if apiKey == "" {
		fmt.Printf("Error: API key not set for provider %s. Use 'ai config' or set the appropriate environment variable.\n", provider)
		os.Exit(1)
	}

	messages := []AIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: textCommand},
	}

	maxAttempts := maxRetries
	attempts := 0

	for attempts < maxAttempts {
		responseText, err := callAPI[string](provider, model, apiKey, messages, LLMRequest)

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

		err = executeCLICommand(cmd.Content)
		if err == nil {
			addToVecDB(vector, textCommand, cmd.Content)
			break
		} else {
			errorMessage := err.Error()
			fmt.Printf("Error executing command: %v\n", errorMessage)
			if len(errorMessage) > 500 {
				errorMessage = errorMessage[:500]
			}
			messages = append(messages, AIMessage{Role: "assistant", Content: cmd.Content})
			messages = append(messages, AIMessage{Role: "user", Content: errorMessage})
			attempts++
		}
	}
}

func confirmExecution() bool {
	requireConfirmation := viper.GetBool("require_confirmation")
	if !requireConfirmation {
		return true
	}

	fmt.Print("Do you want to execute this command? (y/n): ")
	var response string
	fmt.Scanln(&response)
	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func executeCLICommand(command string) error {
	if viper.GetBool("require_confirmation") {
		if !confirmExecution() {
			fmt.Println("Command execution cancelled.")
			return nil
		}
	}

	fmt.Println("Executing command...")

	//Workaround for changing directories
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		dir := strings.TrimSpace(strings.TrimPrefix(command, "cd "))
		fmt.Printf("Your directory cannot be changed. Run: \ncd %s\n", dir)
		return nil
	}

	execCmd := exec.Command("sh", "-c", command)
	var stderr bytes.Buffer
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("%s", strings.TrimSpace(errMsg))
	}
	return nil
}
