package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

func getOpenAIClient() (*openai.Client, error) {
	key := viper.GetString("api_keys.openai")
	if key == "" {
		key = os.Getenv("OPENAI_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("OpenAI API key not found. Please set it in the config or as an environment variable")
	}
	return openai.NewClient(key), nil
}

func openaiCompletion(messages []AIMessage, model string, maxTokens int) (string, error) {
	client, err := getOpenAIClient()
	if err != nil {
		return "", err
	}

	openaiMessages, err := convertToOpenAIMessages(messages)
	if err != nil {
		return "", fmt.Errorf("Failed to convert messages to OpenAI format")
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:     model,
			Messages:  openaiMessages,
			MaxTokens: maxTokens,
		},
	)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}
	return resp.Choices[0].Message.Content, nil
}

func openaiStream(messages []AIMessage, model string, maxTokens int, onContent func(string)) (string, error) {
	client, err := getOpenAIClient()
	if err != nil {
		return "", err
	}

	openaiMessages, err := convertToOpenAIMessages(messages)
	if err != nil {
		return "", fmt.Errorf("Failed to convert messages to OpenAI format")
	}

	stream, err := client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:     model,
			Messages:  openaiMessages,
			MaxTokens: maxTokens,
			Stream:    true,
		},
	)
	if err != nil {
		return "", fmt.Errorf("OpenAI stream error: %v", err)
	}
	defer stream.Close()

	var fullResponse string
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("OpenAI stream error: %v", err)
		}
		content := response.Choices[0].Delta.Content
		onContent(content)
		fullResponse += content
	}
	return fullResponse, nil
}

func convertToOpenAIMessages(messages []AIMessage) ([]openai.ChatCompletionMessage, error) {
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		openaiMessage := openai.ChatCompletionMessage{
			Content: msg.Content,
		}
		switch msg.Role {
		case "user":
			openaiMessage.Role = openai.ChatMessageRoleUser
		case "assistant":
			openaiMessage.Role = openai.ChatMessageRoleAssistant
		case "system":
			openaiMessage.Role = openai.ChatMessageRoleSystem
		default:
			return nil, fmt.Errorf("unknown message role: %s", msg.Role)
		}
		openaiMessages = append(openaiMessages, openaiMessage)
	}
	return openaiMessages, nil
}
