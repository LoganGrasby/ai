package cmd

import (
	"bufio"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
	badger "github.com/dgraph-io/badger/v4"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	usearch "github.com/unum-cloud/usearch/golang"
)

const (
	configFileName = "ai-config.yaml"
	cacheFileName  = "ai_cache.json"
	indexFileName  = "index.usearch"
)

var (
	storeDir, currentDir = getDir()
	indexFile            = filepath.Join(storeDir, indexFileName)
	configFile           = filepath.Join(storeDir, configFileName)
	osInfo               = runtime.GOOS
	maxTokens            int
	temperature          float64
	k                    int
	defaultMaxDistance   float64
	vectorSize           int
)

var systemPrompt = fmt.Sprintf(`
You are an AI CLI assistant that explains your reasoning step by step, incorporating dynamic Chain of Thought (CoT), reflection, and verbal reinforcement learning.
You specialize in solving coding problems. You have direct access to the computers terminal and can execute commands.
Follow these instructions:

1. Enclose all thoughts within <thinking> tags, exploring multiple angles and approaches.
2. Output the command within XML tags like this: <command>CLI command</command> when you want to see the result of the command.
3. If you don't need to review the results of the command to solve the problem output <final_command>CLI command</final_command> instead.
4. Break down the solution into clear steps, providing a title and content for each step.
5. After each step, decide if you need another step or if you're ready to give the final answer.
6. Continuously adjust your reasoning based on intermediate results and reflections, adapting your strategy as you progress.
7. Regularly evaluate your progress, being critical and honest about your reasoning process.
8. Assign a quality score between 0.0 and 1.0 to guide your approach:
   - 0.8+: Continue current approach
   - 0.5-0.7: Consider minor adjustments
   - Below 0.5: Seriously consider backtracking and trying a different approach
9. If unsure or if your score is low, backtrack and try a different approach, explaining your decision.
10. Explore multiple solutions individually if possible, comparing approaches in your reflections.
11. Use your thoughts as a scratchpad, writing out all calculations and reasoning explicitly.
12. Review whether syntax errors exist

Here is the problem statement:
Translate the following text command to a CLI command for %s.
The current working directory is %s.
`, osInfo, currentDir)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "ai",
	Short: "Generate a command with an LLM",
	Long:  `A longer description that spans multiple lines...`,
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		execute(args)
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.TraverseChildren = true
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "Configuration Menu", "Open the configuration menu")
}

func getDir() (string, string) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	if gopath == "" {
		log.Fatal("GOPATH is not set and couldn't be determined")
	}
	storeDir := filepath.Join(gopath, "src", "github.com", "LoganGrasby", "ai")
	err := os.MkdirAll(storeDir, 0755)
	if err != nil {
		log.Fatalf("Error creating directory: %v", err)
	}
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}
	return storeDir, currentDir
}

func initBadgerDB() error {
	var err error
	opts := badger.DefaultOptions(storeDir)
	opts.MemTableSize = 64 << 20
	opts.ValueLogFileSize = 1 << 30
	opts.NumMemtables = 2
	opts.NumLevelZeroTables = 2
	opts.NumLevelZeroTablesStall = 4
	opts.ValueThreshold = 1 << 10
	opts.Logger = nil
	db, err = badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %v", err)
	}
	return nil
}

func initIndex() {
	vectorSize := viper.GetInt("vector_size")
	conf := usearch.DefaultConfig(uint(vectorSize))
	var err error
	index, err = usearch.NewIndex(conf)

	if err != nil {
		panic(fmt.Sprintf("Failed to create Index: %v", err))
	}
	err = index.Load(indexFile)
	if err == nil {
		return
	}
	err = index.Save(indexFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to save index: %v", err))
	}
	fmt.Println("New index created and saved successfully.")
}

func initConfig() {
	viper.SetConfigName(configFileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(storeDir)

	viper.SetDefault("require_confirmation", false)
	viper.SetDefault("k", 5)
	viper.SetDefault("max_distance", 0.5)
	viper.SetDefault("vector_size", 1536)
	viper.SetDefault("max_tokens", 1000)
	viper.SetDefault("temperature", 0.1)
	viper.SetDefault("stream", true)

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("No config file found. Using defaults.")
		} else {
			fmt.Printf("Error reading config file: %v\n", err)
		}
	}

	maxTokens = viper.GetInt("max_tokens")
	temperature = viper.GetFloat64("temperature")
	k = viper.GetInt("k")
	defaultMaxDistance = viper.GetFloat64("max_distance")
	vectorSize = viper.GetInt("vector_size")
	err := initBadgerDB()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize BadgerDB: %v", err))
	}
	initIndex()
}

func execute(args []string) {
	log.SetLevel(map[bool]log.Level{true: log.DebugLevel, false: log.InfoLevel}[false])
	var fullCommand string

	if len(args) > 0 {
		fullCommand = strings.Join(args, " ")
	} else {
		fmt.Print(termenv.String("Enter your command: ").Bold())
		reader := bufio.NewReader(os.Stdin)
		cmdInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			os.Exit(1)
		}
		fullCommand = strings.TrimSpace(cmdInput)
	}
	executeCommand(fullCommand)
	defer db.Close()
	defer index.Destroy()
}

func openFileExplorer(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
