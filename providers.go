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

type AnthropicRequest struct {
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
	Response string `json:"response"`
}

func callAPI(provider, model, apiKey, prompt string) (string, error) {
	var apiURL string
	var req interface{}
	headers := make(map[string]string)
	var processResponse func([]byte) (string, error)

	switch provider {
	case "Anthropic":
		apiURL = "https://api.anthropic.com/v1/messages"
		req = AnthropicRequest{
			Messages:    []AIMessage{{Role: "user", Content: prompt}},
			Model:       model,
			MaxTokens:   1000,
			Temperature: 0.1,
		}
		headers["x-api-key"] = apiKey
		headers["Content-Type"] = "application/json"
		headers["anthropic-version"] = "2023-06-01"
		processResponse = func(body []byte) (string, error) {
			var apiResp AnthropicResponse
			err := json.Unmarshal(body, &apiResp)
			if err != nil {
				return "", fmt.Errorf("Error unmarshaling JSON: %v", err)
			}
			if apiResp.Content[0].Text == "" {
				return "", fmt.Errorf("Error: The LLM returned an empty response")
			}
			return apiResp.Content[0].Text, nil
		}
	case "OpenAI":
		apiURL = "https://api.openai.com/v1/chat/completions"
		req = OpenAIRequest{
			Model:       model,
			Messages:    []AIMessage{{Role: "user", Content: prompt}},
			Temperature: 0.7,
		}
		headers["Authorization"] = "Bearer " + apiKey
		headers["Content-Type"] = "application/json"
		processResponse = func(body []byte) (string, error) {
			var apiResp OpenAIResponse
			err := json.Unmarshal(body, &apiResp)
			if err != nil {
				return "", fmt.Errorf("Error unmarshaling JSON: %v", err)
			}
			if len(apiResp.Choices) == 0 {
				return "", fmt.Errorf("No choices in the API response")
			}
			return apiResp.Choices[0].Message.Content, nil
		}
	case "Ollama":
		apiURL = "http://localhost:11434/v1/chat/completions"
		req = OpenAIRequest{
			Model:       model,
			Messages:    []AIMessage{{Role: "user", Content: prompt}},
			Temperature: 0.7,
		}
		headers["Authorization"] = "Bearer " + apiKey
		headers["Content-Type"] = "application/json"
		processResponse = func(body []byte) (string, error) {
			var apiResp OpenAIResponse
			err := json.Unmarshal(body, &apiResp)
			if err != nil {
				return "", fmt.Errorf("Error unmarshaling JSON: %v", err)
			}
			if len(apiResp.Choices) == 0 {
				return "", fmt.Errorf("No choices in the API response")
			}
			return apiResp.Choices[0].Message.Content, nil
		}
	case "Cloudflare":
		accountID := viper.GetString("cloudflare_account_id")
		if accountID == "" {
			accountID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		}
		if accountID == "" {
			return "", fmt.Errorf("Cloudflare Account ID not set. Use 'ai config' or set the CLOUDFLARE_ACCOUNT_ID environment variable")
		}
		apiURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", accountID, model)
		req = CloudflareRequest{
			Messages:    []AIMessage{{Role: "user", Content: prompt}},
			MaxTokens:   500,
			Temperature: 0.6,
		}
		headers["Authorization"] = "Bearer " + apiKey
		headers["Content-Type"] = "application/json"
		processResponse = func(body []byte) (string, error) {
			var apiResp CloudflareResponse
			err := json.Unmarshal(body, &apiResp)
			if err != nil {
				return "", fmt.Errorf("Error unmarshaling JSON: %v", err)
			}
			fmt.Printf("DEBUG: Unmarshaled API response: %+v\n", apiResp)
			if apiResp.Response == "" {
				return "", fmt.Errorf("No result in the API response")
			}
			return apiResp.Response, nil
		}
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	return makeAPICall(apiURL, req, headers, processResponse)
}

func makeAPICall(apiURL string, req interface{}, headers map[string]string, processResponse func([]byte) (string, error)) (string, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("Error marshaling JSON: %v", err)
	}

	client := &http.Client{}
	request, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("Error creating request: %v", err)
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	resp, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("Error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, body)
	}

	return processResponse(body)
}
