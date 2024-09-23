package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
	mainMenuList       list.Model
	textInput          textinput.Model
	statusMessage      string
	quitting           bool
	currentStep        int
	selectedProvider   string
	selectedLLM        string
	apiKeyRequired     bool
	accountID          string
	settings           []string
	providerList       list.Model
	llmList            list.Model
	toggleConfirmation list.Model
	currentList        *list.Model
	activeList         *list.Model
}

const (
	StepNone = iota
	StepSelectProvider
	StepSelectLLM
	StepEnterAPIKey
	StepEnterAccountID
	StepToggleConfirmation
)

func initialModel() configModel {
	mainMenuItems := []list.Item{
		item{title: "Configure Provider and API Key", desc: "Select provider, LLM, and configure API Key"},
		item{title: "Toggle Command Confirmation", desc: "Enable/Disable command confirmation"},
		item{title: "Advanced Settings", desc: "LLM and vector index settings"},
		item{title: "View Current Configuration", desc: "Display current settings"},
		item{title: "Save and Exit", desc: "Save changes and exit"},
	}

	mainMenuList := list.New(mainMenuItems, list.NewDefaultDelegate(), 0, 0)
	mainMenuList.Title = "AI Config"
	mainMenuList.SetShowStatusBar(false)
	mainMenuList.SetFilteringEnabled(false)
	mainMenuList.SetShowHelp(false)

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
	providerItems := []list.Item{}
	for _, provider := range providerChoices {
		providerItems = append(providerItems, item{title: provider})
	}
	providerList := list.New(providerItems, list.NewDefaultDelegate(), 0, 0)
	providerList.Title = "Select Provider"
	providerList.SetShowStatusBar(false)
	providerList.SetFilteringEnabled(false)
	providerList.SetShowHelp(false)

	toggleItems := []list.Item{
		item{title: "Enable"},
		item{title: "Disable"},
	}
	toggleList := list.New(toggleItems, list.NewDefaultDelegate(), 0, 0)
	toggleList.Title = "Toggle Command Confirmation"
	toggleList.SetShowStatusBar(false)
	toggleList.SetFilteringEnabled(false)
	toggleList.SetShowHelp(false)
	llmItems := []list.Item{}
	llmList := list.New(llmItems, list.NewDefaultDelegate(), 0, 0)
	llmList.Title = "Select LLM"
	llmList.SetShowStatusBar(false)
	llmList.SetFilteringEnabled(false)
	llmList.SetShowHelp(false)

	return configModel{
		mainMenuList:       mainMenuList,
		textInput:          ti,
		currentStep:        StepNone,
		selectedProvider:   selectedProvider,
		selectedLLM:        selectedLLM,
		accountID:          viper.GetString("cloudflare_account_id"),
		settings:           []string{"temperature", "max_tokens", "vector_size", "k", "max_distance", "embeddings_provider", "embeddings_model"},
		providerList:       providerList,
		llmList:            llmList,
		toggleConfirmation: toggleList,
		activeList:         &mainMenuList,
	}
}

func (m configModel) Init() tea.Cmd {
	return nil
}

func (m *configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

		switch m.currentStep {
		case StepNone:
			m.mainMenuList, cmd = m.mainMenuList.Update(msg)
			cmds = append(cmds, cmd)
			if msg.Type == tea.KeyEnter {
				return m.handleMainMenuSelection()
			}
		case StepSelectProvider:
			m.providerList, cmd = m.providerList.Update(msg)
			cmds = append(cmds, cmd)
			if msg.Type == tea.KeyEnter {
				m.handleListSelection(&m.providerList, &m.selectedProvider, StepSelectLLM)
				m.llmList = createLLMList(m.selectedProvider)
				m.activeList = &m.llmList
			}
		case StepSelectLLM:
			m.llmList, cmd = m.llmList.Update(msg)
			cmds = append(cmds, cmd)
			if msg.Type == tea.KeyEnter {
				m.handleListSelection(&m.llmList, &m.selectedLLM, StepEnterAccountID)
				return m.handleLLMSelection()
			}
		case StepToggleConfirmation:
			m.toggleConfirmation, cmd = m.toggleConfirmation.Update(msg)
			cmds = append(cmds, cmd)
			if msg.Type == tea.KeyEnter {
				m.handleToggleConfirmationSelection()
				m.currentStep = StepNone
				m.activeList = &m.mainMenuList
			}
		case StepEnterAccountID, StepEnterAPIKey:
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
			if m.handleTextInput(msg) {
				m.activeList = &m.mainMenuList
			}
		}

		if msg.Type == tea.KeyEsc && m.currentStep != StepNone {
			m.currentStep = StepNone
			m.activeList = &m.mainMenuList
		}

	case tea.WindowSizeMsg:
		m.mainMenuList.SetSize(msg.Width, msg.Height-4)
		m.providerList.SetSize(msg.Width, msg.Height-4)
		m.llmList.SetSize(msg.Width, msg.Height-4)
		m.toggleConfirmation.SetSize(msg.Width, msg.Height-4)
	}

	return m, tea.Batch(cmds...)
}

func (m *configModel) handleListSelection(listModel *list.Model, selection *string, nextStep int) bool {
	if listModel.SelectedItem() != nil {
		switch listModel.SelectedItem().(type) {
		case item:
			i := listModel.SelectedItem().(item)
			*selection = i.title
			m.currentStep = nextStep
			return true
		}
	}
	return false
}

func (m *configModel) handleMainMenuSelection() (tea.Model, tea.Cmd) {
	if m.mainMenuList.SelectedItem() != nil {
		selected := m.mainMenuList.SelectedItem().(item).title
		switch selected {
		case "Configure Provider and API Key":
			m.currentStep = StepSelectProvider
			m.activeList = &m.providerList
		case "Toggle Command Confirmation":
			m.currentStep = StepToggleConfirmation
			m.activeList = &m.toggleConfirmation

			if viper.GetBool("require_confirmation") {
				m.toggleConfirmation.Select(0)
			} else {
				m.toggleConfirmation.Select(1)
			}
		case "View Current Configuration":
			apiKey := getAPIKey(m.selectedProvider)
			m.statusMessage = fmt.Sprintf("Provider: %s\nModel: %s\nAPI Key: %s\nRequire Confirmation: %v",
				m.selectedProvider, m.selectedLLM, maskAPIKey(apiKey), viper.GetBool("require_confirmation"))
		case "Save and Exit":
			err := saveConfig()
			if err != nil {
				m.statusMessage = err.Error()
			} else {
				m.statusMessage = "Configuration saved successfully."
			}
			m.quitting = true
			return m, tea.Quit
		}
	}
	err := saveConfig()
	if err != nil {
		m.statusMessage = err.Error()
	}
	return m, nil
}

func (m *configModel) handleLLMSelection() (tea.Model, tea.Cmd) {
	if m.selectedProvider == "Cloudflare" {
		m.currentStep = StepEnterAccountID
		m.textInput.Placeholder = "Enter Cloudflare Account ID"
		m.textInput.SetValue(m.accountID)
		m.textInput.CursorEnd()
		m.textInput.Focus()
	} else if !apiKeyAvailable(m.selectedProvider) {
		m.apiKeyRequired = true
		m.currentStep = StepEnterAPIKey
		existingAPIKey := getAPIKey(m.selectedProvider)
		if existingAPIKey != "" {
			maskedAPIKey := maskAPIKey(existingAPIKey)
			m.textInput.SetValue(maskedAPIKey)
			m.textInput.CursorEnd()
		} else {
			m.textInput.Placeholder = "Enter API Key"
		}
		m.textInput.Focus()
	} else {
		m.apiKeyRequired = false
		m.currentStep = StepNone
		m.statusMessage = "Provider and LLM configured."
		viper.Set("provider", m.selectedProvider)
		viper.Set("model", m.selectedLLM)
	}
	return m, nil
}

func (m *configModel) handleToggleConfirmationSelection() bool {
	if m.toggleConfirmation.SelectedItem() != nil {
		m.toggleConfirmation.SetShowStatusBar(false)
		m.toggleConfirmation.SetFilteringEnabled(false)
		m.toggleConfirmation.SetShowHelp(false)
		m.toggleConfirmation.SetWidth(30)
		choice := m.toggleConfirmation.SelectedItem().(item).title
		if choice == "Enable" {
			viper.Set("require_confirmation", true)
			m.statusMessage = "Require Confirmation enabled."
		} else {
			viper.Set("require_confirmation", false)
			m.statusMessage = "Require Confirmation disabled."
		}
		return true
	}
	return false
}

func (m *configModel) handleTextInput(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyEnter:
		switch m.currentStep {
		case StepEnterAccountID:
			m.accountID = m.textInput.Value()
			if m.accountID != "" {
				viper.Set("cloudflare_account_id", m.accountID)
			}
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Enter API Key"
			m.currentStep = StepEnterAPIKey
		case StepEnterAPIKey:
			apiKey := m.textInput.Value()
			if apiKey != "" {
				saveAPIKey(m.selectedProvider, apiKey, m.accountID)
			}
			m.apiKeyRequired = false
			m.currentStep = StepNone
			m.textInput.Blur()
			m.statusMessage = "API Key saved. Provider and LLM configured."
			viper.Set("provider", m.selectedProvider)
			viper.Set("model", m.selectedLLM)
		}
		return true
	case tea.KeyEsc:
		m.currentStep = StepNone
		m.textInput.Blur()
		return true
	}
	return false
}

func (m configModel) View() string {
	if m.quitting {
		return m.statusMessage
	}

	var content string
	// var title string
	var footer string

	switch m.currentStep {
	case StepNone:
		content = m.mainMenuList.View()
	case StepSelectProvider:
		content = m.providerList.View()
	case StepSelectLLM:
		content = m.llmList.View()
	case StepToggleConfirmation:
		content = m.toggleConfirmation.View()
	case StepEnterAccountID:
		content = lipgloss.NewStyle().Margin(1, 0, 1, 4).Render(m.textInput.View())
		footer = "(Press Enter to confirm, Esc to cancel)"
	case StepEnterAPIKey:
		content = lipgloss.NewStyle().Margin(1, 0, 1, 4).Render(m.textInput.View())
		footer = "(Press Enter to confirm, Esc to cancel)"
	}

	// titleStyle := lipgloss.NewStyle().
	// 	Foreground(lipgloss.Color("205")).
	// 	Bold(true).
	// 	Margin(1, 0, 1, 2)

	contentStyle := lipgloss.NewStyle().
		Margin(0, 2)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Margin(1, 0, 0, 2)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Margin(1, 0, 0, 2)

	view := contentStyle.Render(content) + "\n"

	if footer != "" {
		view += footerStyle.Render(footer) + "\n"
	}

	if m.statusMessage != "" {
		view += statusStyle.Render(m.statusMessage)
	}

	return view
}

func createLLMList(provider string) list.Model {
	llmItems := []list.Item{}
	for _, llm := range getLLMsForProvider(provider) {
		llmItems = append(llmItems, item{title: llm})
	}
	llmList := list.New(llmItems, list.NewDefaultDelegate(), 0, 0)
	llmList.Title = "Select LLM"
	llmList.SetShowStatusBar(false)
	llmList.SetFilteringEnabled(false)
	llmList.SetShowHelp(false)
	llmList.SetWidth(30)
	return llmList
}

func loadConfig() error {
	viper.SetConfigName(configFileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(storeDir)
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

	initialModelPtr := initialModel()

	p := tea.NewProgram(&initialModelPtr)

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
		if apiKey != "" {
			viper.Set("api_keys.anthropic", apiKey)
		}
	case "OpenAI":
		if apiKey != "" {
			viper.Set("api_keys.openai", apiKey)
		}
	case "Cloudflare":
		if apiKey != "" {
			viper.Set("api_keys.cloudflare", apiKey)
		}
		if accountID != "" {
			viper.Set("cloudflare_account_id", accountID)
		}
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
	err := viper.WriteConfigAs(configFile)
	if err != nil {
		return fmt.Errorf("Error saving config file: %v", err)
	}
	return nil
}

func openConfigDirectory() {
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = exec.Command("open", storeDir).Start()
	case "windows":
		err = exec.Command("explorer", storeDir).Start()
	case "linux":
		err = exec.Command("xdg-open", storeDir).Start()
	default:
		fmt.Printf("Unsupported operating system: %s\n", runtime.GOOS)
		return
	}

	if err != nil {
		fmt.Printf("Error opening config directory: %v\n", err)
	} else {
		fmt.Printf("Opened config directory: %s\n", storeDir)
	}
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View or modify the configuration settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		showConfig, _ := cmd.Flags().GetBool("show")
		if showConfig {
			openConfigDirectory()
		} else {
			configMenu()
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().Bool("show", false, "Open the configuration directory in file explorer")
}
