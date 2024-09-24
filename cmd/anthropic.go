package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/liushuangls/go-anthropic/v2"
	"github.com/spf13/viper"
)

type Model string

func getAnthropicClient() (*anthropic.Client, error) {
	key := viper.GetString("api_keys.anthropic")
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("Anthropic API key not found. Please set it in the config or as an environment variable")
	}
	return anthropic.NewClient(key), nil
}

func anthropicCompletion(messages []AIMessage, model string, maxTokens int) (string, error) {
	client, err := getAnthropicClient()
	if err != nil {
		return "", err
	}
	anthropicMessages, err := convertToAnthropicMessages(messages)
	if err != nil {
		return "", fmt.Errorf("Failed to convert messages to Anthropic format")
	}

	resp, err := client.CreateMessages(context.Background(),
		anthropic.MessagesRequest{
			Model:     anthropic.Model(model),
			Messages:  anthropicMessages,
			MaxTokens: maxTokens,
		})
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %v", err)
	}
	return resp.Content[0].GetText(), nil
}

func anthropicStream(messages []AIMessage, model string, maxTokens int, onContent func(string)) (string, error) {
	client, err := getAnthropicClient()
	if err != nil {
		return "", err
	}

	anthropicMessages, err := convertToAnthropicMessages(messages)

	resp, err := client.CreateMessagesStream(context.Background(), anthropic.MessagesStreamRequest{
		MessagesRequest: anthropic.MessagesRequest{
			Model:     anthropic.Model(model),
			Messages:  anthropicMessages,
			MaxTokens: maxTokens,
		},
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			if data.Delta.Text != nil {
				onContent(*data.Delta.Text)
			}
		},
	})
	if err != nil {
		var e *anthropic.APIError
		if errors.As(err, &e) {
			fmt.Errorf("Anthropic stream error, type: %s, message: %s", e.Type, e.Message)
		}
		fmt.Errorf("Anthropic stream error: %v", err)
	}
	return resp.Content[0].GetText(), nil
}

func convertToAnthropicMessages(messages []AIMessage) ([]anthropic.Message, error) {
	anthropicMessages := make([]anthropic.Message, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			anthropicMessages = append(anthropicMessages, anthropic.NewUserTextMessage(msg.Content))
		case "assistant":
			anthropicMessages = append(anthropicMessages, anthropic.NewAssistantTextMessage(msg.Content))
		default:
			return nil, fmt.Errorf("unknown message role: %s", msg.Role)
		}
	}
	return anthropicMessages, nil
}
