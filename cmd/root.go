package cmd

import (
	"bufio"
	"fmt"
	"go/build"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	usearch "github.com/unum-cloud/usearch/golang"

	badger "github.com/dgraph-io/badger/v4"
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

var systemPrompt = fmt.Sprintf("Translate the following text command to a CLI command for %s. The current working directory is %s. Output the command within XML tags like this: <command>CLI command</command>", osInfo, currentDir)

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

	viper.SetDefault("require_confirmation", true)
	viper.SetDefault("k", 5)
	viper.SetDefault("max_distance", 0.5)
	viper.SetDefault("vector_size", 1536)
	viper.SetDefault("max_tokens", 1000)
	viper.SetDefault("temperature", 0.1)

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

	var fullCommand string

	if len(args) > 0 {
		fullCommand = strings.Join(args, " ")
	} else {
		fmt.Print("Enter your command: ")
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
