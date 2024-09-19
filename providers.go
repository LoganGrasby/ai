package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/viper"
)

type Command struct {
	XMLName xml.Name `xml:"command"`
	Content string   `xml:",chardata"`
}

type AIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model       string      `json:"model"`
	Messages    []AIMessage `json:"messages"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature,omitempty"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
type OpenAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type AnthropicRequest struct {
	System      string      `json:"system"`
	Messages    []AIMessage `json:"messages"`
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature,omitempty"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type CloudflareRequest struct {
	Messages    []AIMessage `json:"messages"`
	MaxTokens   int         `json:"max_tokens"`
	Temperature float64     `json:"temperature,omitempty"`
}

type CloudflareResponse struct {
	Result struct {
		Response string `json:"response"`
	} `json:"result"`
}

type OllamaEmbeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type OllamaEmbeddingResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	TotalDuration   int64       `json:"total_duration"`
	LoadDuration    int64       `json:"load_duration"`
	PromptEvalCount int         `json:"prompt_eval_count"`
}

func callAPI[T any](provider, model, apiKey string, input interface{}, requestType RequestType) (T, error) {
	var (
		apiURL          string
		req             interface{}
		headers         = map[string]string{"Content-Type": "application/json"}
		processResponse func([]byte) (T, error)
	)

	switch provider {
	case "Anthropic":
		apiURL = "https://api.anthropic.com/v1/messages"
		headers["x-api-key"] = apiKey
		headers["anthropic-version"] = "2023-06-01"

		messages, ok := input.([]AIMessage)
		if !ok {
			var zero T
			return zero, fmt.Errorf("Invalid input type for Anthropic")
		}
		filteredMessages := filterSystemMessages(messages)

		req = AnthropicRequest{
			System:      systemPrompt,
			Messages:    filteredMessages,
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}

		processResponse = func(body []byte) (T, error) {
			var apiResp AnthropicResponse
			if err := json.Unmarshal(body, &apiResp); err != nil {
				var zero T
				return zero, fmt.Errorf("Error unmarshaling JSON: %v", err)
			}
			if len(apiResp.Content) == 0 || apiResp.Content[0].Text == "" {
				var zero T
				return zero, fmt.Errorf("The LLM returned an empty response")
			}
			return any(apiResp.Content[0].Text).(T), nil
		}

	case "OpenAI", "Ollama":
		baseURL := map[string]string{
			"OpenAI": "https://api.openai.com",
			"Ollama": "http://localhost:11434",
		}[provider]

		if provider == "OpenAI" {
			headers["Authorization"] = "Bearer " + apiKey
		}

		switch requestType {
		case LLMRequest:
			apiURL = baseURL + "/v1/chat/completions"
			messages, ok := input.([]AIMessage)
			if !ok {
				var zero T
				return zero, fmt.Errorf("Invalid input type for LLM request")
			}
			req = OpenAIRequest{
				Model:       model,
				Messages:    messages,
				MaxTokens:   maxTokens,
				Temperature: temperature,
			}
			processResponse = processLLMResponse[T]

		// case EmbeddingsRequest:
		// 	apiURL = baseURL + "/v1/embeddings"
		// 	req = OpenAIRequest{
		// 		Model: model,
		// 		Input: input,
		// 	}
		// 	processResponse = processEmbeddingResponse[T]

		default:
			var zero T
			return zero, fmt.Errorf("Invalid request type for %s provider", provider)
		}

	case "Cloudflare":
		accountID := viper.GetString("cloudflare_account_id")
		if accountID == "" {
			accountID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		}
		if accountID == "" {
			var zero T
			return zero, fmt.Errorf("Cloudflare Account ID not set. Use 'ai config' or set the CLOUDFLARE_ACCOUNT_ID environment variable")
		}
		apiURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", accountID, model)
		headers["Authorization"] = "Bearer " + apiKey

		messages, ok := input.([]AIMessage)
		if !ok {
			var zero T
			return zero, fmt.Errorf("Invalid input type for Cloudflare")
		}
		req = CloudflareRequest{
			Messages:    messages,
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}

		processResponse = func(body []byte) (T, error) {
			var apiResp CloudflareResponse
			if err := json.Unmarshal(body, &apiResp); err != nil {
				var zero T
				return zero, fmt.Errorf("Error unmarshaling JSON: %v", err)
			}
			if apiResp.Result.Response == "" {
				var zero T
				return zero, fmt.Errorf("No result in the API response")
			}
			return any(apiResp.Result.Response).(T), nil
		}

	default:
		var zero T
		return zero, fmt.Errorf("Unsupported provider: %s", provider)
	}

	return makeAPICall(apiURL, req, headers, processResponse)
}

func filterSystemMessages(messages []AIMessage) []AIMessage {
	var filtered []AIMessage
	for _, msg := range messages {
		if msg.Role != "system" {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

func processLLMResponse[T any](body []byte) (T, error) {
	var apiResp OpenAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		var zero T
		return zero, fmt.Errorf("Error unmarshaling JSON: %v", err)
	}
	if len(apiResp.Choices) == 0 {
		var zero T
		return zero, fmt.Errorf("No choices in the API response")
	}
	content := apiResp.Choices[0].Message.Content
	return any(content).(T), nil
}

func processEmbeddingResponse[T any](body []byte) (T, error) {
	var apiResp OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		var zero T
		return zero, fmt.Errorf("Error unmarshaling JSON: %v", err)
	}
	if len(apiResp.Data) == 0 {
		var zero T
		return zero, fmt.Errorf("No embeddings returned in the API response")
	}
	embeddings := make([][]float64, len(apiResp.Data))
	for i, data := range apiResp.Data {
		embeddings[i] = data.Embedding
	}
	return any(embeddings).(T), nil
}

func makeAPICall[T any](apiURL string, req interface{}, headers map[string]string, processResponse func([]byte) (T, error)) (T, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("Error marshaling JSON: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		var zero T
		return zero, fmt.Errorf("Error creating request: %v", err)
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	resp, err := client.Do(request)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("Error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var zero T
		return zero, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	return processResponse(body)
}
