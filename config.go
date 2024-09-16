package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

type OllamaModel struct {
	Name string `json:"name"`
}

type OllamaResponse struct {
	Models []OllamaModel `json:"models"`
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

var toggleChoices = []string{"Enable", "Disable"}

type configModel struct {
	list          list.Model
	textInput     textinput.Model
	selectedItem  string
	statusMessage string
	quitting      bool

	toggleCursor int

	currentStep      int
	providerChoices  []string
	providerCursor   int
	llmChoices       []string
	llmCursor        int
	selectedProvider string
	selectedLLM      string
	apiKeyRequired   bool
	accountID        string
}

const (
	StepNone = iota
	StepSelectProvider
	StepSelectLLM
	StepEnterAPIKey
	StepEnterAccountID
)

func initialModel() configModel {
	items := []list.Item{
		item{title: "Configure Provider and API Key", desc: "Select provider, LLM, and configure API Key"},
		item{title: "Toggle Command Confirmation", desc: "Enable/Disable command confirmation"},
		item{title: "View Current Configuration", desc: "Display current settings"},
		item{title: "Save and Exit", desc: "Save changes and exit"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "AI Config"

	ti := textinput.New()
	ti.Placeholder = "Enter API Key"

	selectedProvider := viper.GetString("provider")
	if selectedProvider == "" {
		selectedProvider = "Anthropic"
	}

	selectedLLM := viper.GetString("model")
	if selectedLLM == "" {
		selectedLLM = "claude-3-5-sonnet-20240620"
	}

	providerChoices := []string{"Cloudflare", "Anthropic", "OpenAI", "Ollama"}

	accountID := viper.GetString("cloudflare_account_id")

	return configModel{
		list:             l,
		textInput:        ti,
		currentStep:      StepNone,
		providerChoices:  providerChoices,
		selectedProvider: selectedProvider,
		selectedLLM:      selectedLLM,
		accountID:        accountID,
	}
}

func (m configModel) Init() tea.Cmd {
	return nil
}

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

		if m.selectedItem == "Configure Provider and API Key" {
			switch m.currentStep {
			case StepSelectProvider:
				switch msg.String() {
				case "up", "k":
					if m.providerCursor > 0 {
						m.providerCursor--
					}
				case "down", "j":
					if m.providerCursor < len(m.providerChoices)-1 {
						m.providerCursor++
					}
				case "enter":
					m.selectedProvider = m.providerChoices[m.providerCursor]
					m.currentStep = StepSelectLLM
					m.llmChoices = getLLMsForProvider(m.selectedProvider)
					m.llmCursor = 0
				case "esc":
					m.selectedItem = ""
					m.currentStep = StepNone
				}
			case StepSelectLLM:
				switch msg.String() {
				case "up", "k":
					if m.llmCursor > 0 {
						m.llmCursor--
					}
				case "down", "j":
					if m.llmCursor < len(m.llmChoices)-1 {
						m.llmCursor++
					}
				case "enter":
					m.selectedLLM = m.llmChoices[m.llmCursor]
					if m.selectedProvider == "Cloudflare" {
						m.currentStep = StepEnterAccountID
						m.textInput.Placeholder = "Enter Cloudflare Account ID"
						m.textInput.Focus()
					} else if !apiKeyAvailable(m.selectedProvider) {
						m.apiKeyRequired = true
						m.currentStep = StepEnterAPIKey
						m.textInput.SetValue("")
						m.textInput.Placeholder = "Enter API Key"
						m.textInput.Focus()
					} else {
						m.apiKeyRequired = false
						m.currentStep = StepNone
						m.selectedItem = ""
						m.statusMessage = "Provider and LLM configured."
						viper.Set("provider", m.selectedProvider)
						viper.Set("model", m.selectedLLM)
					}
					return m, nil
				case "esc":
					m.currentStep = StepSelectProvider
				}
			case StepEnterAccountID:
				switch msg.Type {
				case tea.KeyEnter:
					m.accountID = m.textInput.Value()
					viper.Set("cloudflare_account_id", m.accountID)
					m.textInput.SetValue("")
					m.textInput.Placeholder = "Enter API Key"
					m.currentStep = StepEnterAPIKey
					return m, textinput.Blink
				case tea.KeyEsc:
					m.currentStep = StepSelectLLM
					m.textInput.Blur()
					return m, nil
				}
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			case StepEnterAPIKey:
				switch msg.Type {
				case tea.KeyEnter:
					apiKey := m.textInput.Value()
					saveAPIKey(m.selectedProvider, apiKey, m.accountID)
					m.apiKeyRequired = false
					m.currentStep = StepNone
					m.selectedItem = ""
					m.textInput.Blur()
					m.statusMessage = "API Key saved. Provider and LLM configured."
					viper.Set("provider", m.selectedProvider)
					viper.Set("model", m.selectedLLM)
					return m, nil
				}

				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
			return m, nil
		} else if m.selectedItem == "Toggle Command Confirmation" {
			switch msg.String() {
			case "enter":
				if m.toggleCursor == 0 {
					viper.Set("require_confirmation", true)
					m.statusMessage = "Require Confirmation enabled."
				} else {
					viper.Set("require_confirmation", false)
					m.statusMessage = "Require Confirmation disabled."
				}
				m.selectedItem = ""
				return m, nil
			case "up", "k":
				if m.toggleCursor > 0 {
					m.toggleCursor--
				}
			case "down", "j":
				if m.toggleCursor < len(toggleChoices)-1 {
					m.toggleCursor++
				}
			case "esc":
				m.selectedItem = ""
				return m, nil
			}
			return m, nil
		} else if m.selectedItem == "" {
			switch msg.Type {
			case tea.KeyEnter:
				selected := m.list.SelectedItem().(item).title
				if selected == "Configure Provider and API Key" {
					m.selectedItem = selected
					m.currentStep = StepSelectProvider
					m.providerCursor = 0
				} else if selected == "Toggle Command Confirmation" {
					m.selectedItem = selected
					if viper.GetBool("require_confirmation") {
						m.toggleCursor = 0 // 'Enable' is at index 0
					} else {
						m.toggleCursor = 1 // 'Disable' is at index 1
					}
					return m, nil
				} else if selected == "View Current Configuration" {
					apiKey := getAPIKey(m.selectedProvider)
					m.statusMessage = fmt.Sprintf("Provider: %s\nModel: %s\nAPI Key: %s\nRequire Confirmation: %v",
						m.selectedProvider, m.selectedLLM, maskAPIKey(apiKey), viper.GetBool("require_confirmation"))
					return m, nil
				} else if selected == "Save and Exit" {
					err := saveConfig()
					if err != nil {
						m.statusMessage = err.Error()
					} else {
						m.statusMessage = "Configuration saved successfully."
					}
					m.quitting = true
					return m, tea.Quit
				}
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	if m.selectedItem == "" {
		m.list, cmd = m.list.Update(msg)
	}

	if m.selectedProvider == "Cloudflare" && m.accountID == "" {
		m.currentStep = StepEnterAccountID
		m.textInput.Placeholder = "Enter Cloudflare Account ID"
		m.textInput.Focus()
		return m, nil
	}

	return m, cmd
}

func (m configModel) View() string {
	if m.quitting {
		if m.statusMessage != "" {
			return m.statusMessage + "\n"
		}
		return "Config saved\n"
	}

	if m.selectedItem == "Configure Provider and API Key" {
		switch m.currentStep {
		case StepSelectProvider:
			s := strings.Builder{}
			s.WriteString("Select Provider:\n\n")
			for i, choice := range m.providerChoices {
				cursor := " "
				if m.providerCursor == i {
					cursor = ">"
				}
				s.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
			}
			s.WriteString("\n(Use up/down to navigate, enter to select)")
			return s.String()
		case StepSelectLLM:
			s := strings.Builder{}
			s.WriteString(fmt.Sprintf("Select LLM for %s:\n\n", m.selectedProvider))
			for i, choice := range m.llmChoices {
				cursor := " "
				if m.llmCursor == i {
					cursor = ">"
				}
				s.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
			}
			s.WriteString("\n(Use up/down to navigate, enter to select)")
			return s.String()
		case StepEnterAccountID:
			return fmt.Sprintf(
				"Enter your Cloudflare Account ID:\n\n%s\n\n(press enter to submit)",
				m.textInput.View(),
			)
		case StepEnterAPIKey:
			return fmt.Sprintf(
				"Enter your %s API Key:\n\n%s\n\n(press enter to submit)",
				m.selectedProvider, m.textInput.View(),
			)
		}
	}

	if m.selectedItem == "Toggle Command Confirmation" {
		s := strings.Builder{}
		s.WriteString("Toggle Command Confirmation:\n\n")
		for i, choice := range toggleChoices {
			cursor := " "
			if m.toggleCursor == i {
				cursor = ">"
			}
			s.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
		}
		s.WriteString("\n(press enter to confirm, esc to cancel)")
		return s.String()
	}

	return docStyle.Render(fmt.Sprintf("%s\n\n%s", m.list.View(), m.statusMessage))
}

func loadConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Error getting user home directory: %v", err)
	}

	viper.SetConfigName(configFileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(home)
	viper.SetDefault("require_confirmation", true)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		} else {
			return fmt.Errorf("Error reading config file: %v", err)
		}
	}
	return nil
}

func configMenu() {
	err := loadConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

func getLLMsForProvider(provider string) []string {
	switch provider {
	case "Anthropic":
		return []string{"claude-3-5-sonnet-20240620"}
	case "OpenAI":
		return []string{"gpt-4o", "gpt-4o-mini"}
	case "Cloudflare":
		return []string{"@cf/meta/llama-3.1-8b-instruct"}
	case "Ollama":
		models, err := getOllamaModels()
		if err != nil {
			fmt.Printf("Error fetching Ollama models: %v\n", err)
			return []string{}
		}
		return models
	default:
		return []string{}
	}
}

func apiKeyAvailable(provider string) bool {
	switch provider {
	case "Anthropic":
		return os.Getenv("ANTHROPIC_API_KEY") != ""
	case "OpenAI":
		return os.Getenv("OPENAI_API_KEY") != ""
	case "Cloudflare":
		return os.Getenv("CLOUDFLARE_API_KEY") != ""
	case "Ollama":
		return isOllamaAvailable()
	default:
		return false
	}
}

func saveAPIKey(provider string, apiKey string, accountID string) {
	switch provider {
	case "Anthropic":
		viper.Set("api_keys.anthropic", apiKey)
	case "OpenAI":
		viper.Set("api_keys.openai", apiKey)
	case "Cloudflare":
		viper.Set("api_keys.cloudflare", apiKey)
		viper.Set("cloudflare_account_id", accountID)
	}
}

func getAPIKey(provider string) string {
	var key string
	switch provider {
	case "Anthropic":
		key = viper.GetString("api_keys.anthropic")
		if key == "" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
	case "OpenAI":
		key = viper.GetString("api_keys.openai")
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
	case "Cloudflare":
		key = viper.GetString("api_keys.cloudflare")
		if key == "" {
			key = os.Getenv("CLOUDFLARE_API_KEY")
		}
	case "Ollama":
		key = "ollama_key_placeholder"
	}
	return key
}

func isOllamaAvailable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	_, err := client.Get("http://localhost:11434/api/tags")
	return err == nil
}

func getOllamaModels() ([]string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	var models []string
	for _, model := range ollamaResp.Models {
		models = append(models, model.Name)
	}
	return models, nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func saveConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Error getting user home directory: %v", err)
	}

	configPath := filepath.Join(home, configFileName)
	err = viper.WriteConfigAs(configPath)
	if err != nil {
		return fmt.Errorf("Error saving config file: %v", err)
	}
	return nil
}
