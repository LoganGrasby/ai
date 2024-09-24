package cmd

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
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

type OpenAIEmbeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type OpenAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

var spinnerProgram *tea.Program
var spinnerRunning bool
var contentBuffer strings.Builder

func processStream(content string) {
	for _, char := range content {
		contentBuffer.WriteRune(char)

		if strings.HasSuffix(contentBuffer.String(), "<thinking>") {
			if !spinnerRunning {
				spinnerRunning = true
				spinnerProgram = tea.NewProgram(functionModel("Thinking...", func() error {
					for spinnerRunning {
						time.Sleep(100 * time.Millisecond)
					}
					return nil
				}))
				go func() {
					if _, err := spinnerProgram.Run(); err != nil {
						fmt.Printf("Error running spinner: %v\n", err)
					}
				}()
			}
		} else if strings.HasSuffix(contentBuffer.String(), "</thinking>") {
			if spinnerRunning {
				spinnerRunning = false
				spinnerProgram.Quit()
			}
		}

		if !spinnerRunning {
			fmt.Print(string(char))
		}
	}
}

func cleanup() {
	if spinnerRunning {
		spinnerRunning = false
		spinnerProgram.Quit()
	}
}

func callAPI[T any](provider, model, apiKey string, input interface{}, requestType RequestType) (T, error) {
	var (
		apiURL          string
		req             interface{}
		headers         = map[string]string{"Content-Type": "application/json"}
		processResponse func([]byte) (T, error)
		stream          = viper.GetBool("stream")
	)

	switch provider {
	case "Anthropic":
		messages, ok := input.([]AIMessage)
		if !ok {
			var zero T
			return zero, fmt.Errorf("Invalid input type for Anthropic")
		}
		filteredMessages := filterSystemMessages(messages)
		if stream {
			result, err := anthropicStream(filteredMessages, model, maxTokens, processStream)
			if err != nil {
				return *new(T), fmt.Errorf("streaming error: %w", err)
			}
			return any(result).(T), nil
		} else {
			result, err := anthropicCompletion(filteredMessages, model, maxTokens)
			if err != nil {
				var zero T
				return zero, err
			}
			return any(result).(T), nil
		}
	case "OpenAI":
		baseURL := map[string]string{
			"OpenAI": "https://api.openai.com",
			"Ollama": "http://localhost:11434",
		}[provider]
		if provider == "OpenAI" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		switch requestType {
		case LLMRequest:
			messages, ok := input.([]AIMessage)
			if !ok {
				var zero T
				return zero, fmt.Errorf("Invalid input type for OpenAI")
			}
			if stream {
				result, err := openaiStream(messages, model, maxTokens, processStream)
				if err != nil {
					cleanup()
					return *new(T), fmt.Errorf("streaming error: %w", err)
				}

				cleanup()
				return any(result).(T), nil
			} else {
				result, err := openaiCompletion(messages, model, maxTokens)
				if err != nil {
					var zero T
					return zero, err
				}
				return any(result).(T), nil
			}
		case EmbeddingsRequest:
			apiURL = baseURL + "/v1/embeddings"
			req = OpenAIEmbeddingRequest{
				Input: input.(string),
				Model: model,
			}
			processResponse = processEmbeddingResponse[T]
		default:
			var zero T
			return zero, fmt.Errorf("Invalid request type for %s provider", provider)
		}
	case "Ollama":
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
		case EmbeddingsRequest:
			apiURL = baseURL + "/v1/embeddings"
			req = OpenAIEmbeddingRequest{
				Input: input.(string),
				Model: model,
			}
			processResponse = processEmbeddingResponse[T]
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
	var resp OpenAIEmbeddingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		var zero T
		return zero, fmt.Errorf("Error unmarshaling embeddings response: %v", err)
	}
	if len(resp.Data) == 0 {
		var zero T
		return zero, fmt.Errorf("No embeddings returned")
	}
	embedding := resp.Data[0].Embedding
	return any(embedding).(T), nil
}

func makeAPICall[T any](apiURL string, req interface{}, headers map[string]string, processResponse func([]byte) (T, error)) (T, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		var zero T
		log.Debug("Error marshaling JSON", "error", err)
		return zero, fmt.Errorf("Error marshaling JSON: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		var zero T
		log.Debug("Error creating request", "error", err)
		return zero, fmt.Errorf("Error creating request: %v", err)
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	resp, err := client.Do(request)
	if err != nil {
		var zero T
		log.Debug("Error sending request", "error", err)
		return zero, fmt.Errorf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		var zero T
		log.Debug("Error reading response", "error", err)
		return zero, fmt.Errorf("Error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var zero T
		log.Debug("API request failed", "statusCode", resp.StatusCode, "body", string(body))
		return zero, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}
	result, err := processResponse(body)
	if err != nil {
		log.Debug("Error processing response", "error", err)
	} else {
		log.Debug("Response processed successfully")
	}
	return result, err
}
