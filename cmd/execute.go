package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/spf13/viper"
)

type RequestType int

const (
	maxRetries             = 3
	LLMRequest RequestType = iota
	EmbeddingsRequest
)

func extractCommandContent(input string) (string, bool) {
	commandStart := strings.Index(input, "<command>")
	commandEnd := strings.Index(input, "</command>")
	finalCommandStart := strings.Index(input, "<final_command>")
	finalCommandEnd := strings.Index(input, "</final_command>")

	if commandStart != -1 && commandEnd != -1 && commandStart < commandEnd {
		return input[commandStart+9 : commandEnd], false
	}
	if finalCommandStart != -1 && finalCommandEnd != -1 && finalCommandStart < finalCommandEnd {
		return input[finalCommandStart+15 : finalCommandEnd], true
	}
	return "", false
}

const (
	maxOutputSize  = 1000000
	commandTimeout = 30 * time.Second
)

func captureCommandOutput(command string) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		dir := strings.TrimSpace(strings.TrimPrefix(command, "cd "))
		return fmt.Sprintf("Your directory cannot be changed. Run: \ncd %s\n", dir), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command execution timed out after %v", commandTimeout)
	}

	if err != nil {
		return "", fmt.Errorf("command execution failed: %v\nStderr: %s", err, limitString(stderr.String(), maxOutputSize/2))
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nStderr: " + stderr.String()
	}
	return limitString(strings.TrimSpace(output), maxOutputSize), nil
}

func limitString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("\n... (output truncated, total length: %d)", len(s))
}

func executeCommand(textCommand string) string {
	var result strings.Builder
	log.Debug("Executing command", "command", textCommand)

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	apiKey := getAPIKey(provider)
	log.Debug("API configuration", "provider", provider, "model", model)

	if apiKey == "" {
		log.Debug("API key not set", "provider", provider)
		return fmt.Sprintf("Error: API key not set for provider %s. Use 'ai config' or set the appropriate environment variable.\n", provider)
	}

	messages := []AIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: textCommand},
	}

	retries := 0
	for {
		log.Debug("Calling API", "provider", provider, "retry", retries)
		responseText, err := callAPI[string](provider, model, apiKey, messages, LLMRequest)

		if err != nil {
			log.Debug("Error calling API", "provider", provider, "error", err)
			return fmt.Sprintf("Error calling %s API: %v\n", provider, err)
		}

		commandContent, isFinalCommand := extractCommandContent(responseText)
		if commandContent == "" {
			log.Debug("Error extracting command from LLM response", "response", responseText)
			return fmt.Sprintf("Error: Unable to extract command from LLM response\nRaw response: %s\n", responseText)
		}

		log.Debug("Generated command", "command", commandContent, "isFinal", isFinalCommand)
		result.WriteString(fmt.Sprintf("Generated command: %s\n", commandContent))

		// Check cache for this specific command
		// cachedResponse, found, _ := getCachedResponse(commandContent)
		// if found {
		// 	log.Debug("Found cached response", "response", cachedResponse)
		// 	result.WriteString(fmt.Sprintf("Cached command: %s\n", cachedResponse))
		// 	err := executeCLICommand(cachedResponse)
		// 	if err != nil {
		// 		log.Debug("Error executing cached command", "error", err)
		// 		result.WriteString(fmt.Sprintf("Error executing cached command: %v\n", err))
		// 	}
		// 	return result.String()
		// }

		err = executeCLICommand(commandContent)
		if err != nil {
			errorMessage := err.Error()
			if len(errorMessage) > 500 {
				errorMessage = errorMessage[:500]
			}
			log.Debug("Error executing command", "error", errorMessage)
			result.WriteString(fmt.Sprintf("Error executing command: %v\n", errorMessage))
			messages = append(messages, AIMessage{Role: "assistant", Content: responseText})
			messages = append(messages, AIMessage{Role: "user", Content: errorMessage})

			retries++
			if retries >= maxRetries {
				log.Debug("Max retries reached", "maxRetries", maxRetries)
				return fmt.Sprintf("Error: Max retries (%d) reached. Unable to execute command successfully.\n", maxRetries)
			}
		} else {
			log.Debug("Command executed successfully", "command", commandContent)
			retries = 0 // Reset retries on successful execution

			// Cache the successful command
			// vector, _ := getEmbedding(commandContent)
			// addToVecDB(vector, commandContent, "Command executed successfully")
			output, _ := captureCommandOutput(commandContent)
			if isFinalCommand {
				break
			}
			messages = append(messages, AIMessage{Role: "assistant", Content: responseText})
			messages = append(messages, AIMessage{Role: "user", Content: fmt.Sprintf("Command executed successfully. Output: %s\nWhat's the next step?", output)})
		}
	}
	return result.String()
}

func confirmExecution() bool {
	requireConfirmation := viper.GetBool("require_confirmation")
	log.Debug("Confirmation required", "required", requireConfirmation)

	if !requireConfirmation {
		return true
	}
	fmt.Print(termenv.String("Do you want to execute this command? (y/n): ").Bold())
	var response string
	fmt.Scanln(&response)
	confirmed := strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
	log.Debug("User confirmation", "confirmed", confirmed)
	return confirmed
}

func executeCLICommand(command string) error {
	// if viper.GetBool("require_confirmation") {
	// 	if !confirmExecution() {
	// 		fmt.Println("Command execution cancelled.")
	// 		return nil
	// 	}
	// }
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
