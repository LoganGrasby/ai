package cmd

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	badger "github.com/dgraph-io/badger/v4"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const (
	functionFolder = "functions"
	functionPrompt = `
	Create a Go function named %s that does the following: %s.
	Include necessary import statements at the top of the code.
	Wrap the entire code (including imports) in <function></function> XML tags.
	Consider the following example:

	<function>
	import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
	)

	func github() {
	    url := "https://api.github.com/search/repositories?q=language:go&sort=stars&order=desc"

	    resp, err := http.Get(url)
	    if err != nil {
	        fmt.Println("Error making HTTP request:", err)
	        return
	    }
	    defer resp.Body.Close()

	    body, err := io.ReadAll(resp.Body)
	    if err != nil {
	        fmt.Println("Error reading response body:", err)
	        return
	    }

	    var result struct {
	        Items []struct {
	            FullName string ` + "`json:\"full_name\"`" + `
	            Stars    int    ` + "`json:\"stargazers_count\"`" + `
	        } ` + "`json:\"items\"`" + `
	    }


	    err = json.Unmarshal(body, &result)
	    if err != nil {
	        fmt.Println("Error parsing JSON:", err)
	        fmt.Println("Response Body:", string(body))
	        return
	    }

	    for i := 0; i < 5 && i < len(result.Items); i++ {
	        fmt.Printf("%d. %s (%d stars)\n", i+1, result.Items[i].FullName, result.Items[i].Stars)
	    }
	}
	</function>

	Do not include package declaration as it will be added automatically.
	`
)

var style = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	Background(lipgloss.Color("#7D56F4")).
	PaddingTop(2).
	PaddingLeft(4).
	Width(22)

type APIError struct {
	Msg string
}

func (e APIError) Error() string {
	return fmt.Sprintf("API error: %s", e.Msg)
}

type FunctionError struct {
	Msg string
}

func (e FunctionError) Error() string {
	return fmt.Sprintf("Function error: %s", e.Msg)
}

func testGeneratedFunction(functionName string) (bool, string, error) {
	log.Debug("Testing generated function", "functionName", functionName)

	var functionCode []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("function:" + functionName))
		if err != nil {
			log.Debug("Error getting function from DB", "error", err)
			return err
		}
		return item.Value(func(val []byte) error {
			functionCode = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		log.Debug("Failed to read function from DB", "error", err)
		return false, "", FunctionError{Msg: fmt.Sprintf("Failed to read function from DB: %v", err)}
	}

	_, err = ExecuteFunction(functionName)
	if err != nil {
		log.Debug("Failed to execute function", "error", err)
		return false, string(functionCode), FunctionError{Msg: fmt.Sprintf("Failed to execute function: %v", err)}
	}

	log.Debug("Function tested successfully", "functionName", functionName)
	return true, string(functionCode), nil
}

func GenerateFunction(functionName, description string) error {
	log.Debug("Generating function", "functionName", functionName, "description", description)

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	apiKey := getAPIKey(provider)
	if apiKey == "" {
		log.Debug("API key not set", "provider", provider)
		return APIError{Msg: fmt.Sprintf("API key not set for provider %s", provider)}
	}

	systemPrompt := fmt.Sprintf(functionPrompt, functionName, description)
	messages := []AIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: description},
	}

	maxAttempts := 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		log.Debug("Attempting to generate function", "attempt", attempt+1, "maxAttempts", maxAttempts)

		responseText, err := callAPI[string](provider, model, apiKey, messages, LLMRequest)
		if err != nil {
			log.Debug("API call failed", "error", err)
			return APIError{Msg: fmt.Sprintf("API call failed: %v", err)}
		}

		functionCode := extractFunctionCode(responseText)
		if functionCode == "" {
			log.Debug("Failed to extract function code from API response")
			return FunctionError{Msg: "Failed to extract function code from API response"}
		}

		messages = append(messages, AIMessage{Role: "assistant", Content: functionCode})
		functionCode = fmt.Sprintf("package main\n\n%s", functionCode)

		err = saveFunction(functionName, functionCode)
		if err != nil {
			log.Debug("Failed to save function", "error", err)
			return FunctionError{Msg: fmt.Sprintf("Failed to save function: %v", err)}
		}

		testResult, savedFunctionCode, testErr := testGeneratedFunction(functionName)
		if testResult {
			log.Debug("Function generated and tested successfully", "functionName", functionName)
			return nil
		}

		log.Debug("Function test failed", "attempt", attempt+1, "error", testErr)
		errorMessage := fmt.Sprintf("Function test failed (attempt %d/%d): %v\nHere's the current function code:\n\n%s\n\nPlease correct the function.",
			attempt+1, maxAttempts, testErr, savedFunctionCode)

		messages = append(messages, AIMessage{Role: "user", Content: errorMessage})
	}

	log.Debug("Failed to generate a working function after max attempts", "maxAttempts", maxAttempts)
	return FunctionError{Msg: fmt.Sprintf("Failed to generate a working function after %d attempts", maxAttempts)}
}

func extractFunctionCode(response string) string {
	start := strings.Index(response, "<function>")
	end := strings.Index(response, "</function>")
	if start == -1 || end == -1 || start > end {
		return ""
	}
	code := response[start+10 : end]
	code = strings.TrimPrefix(code, "```go")
	code = strings.TrimPrefix(code, "```")
	code = strings.TrimSuffix(code, "```")
	return strings.TrimSpace(code)
}

func saveFunction(name, code string) error {
	return db.Update(func(txn *badger.Txn) error {
		key := []byte("function:" + name)
		return txn.Set(key, []byte(code))
	})
}

func listFunctions() ([]string, error) {
	var functions []string
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := []byte("function:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			functions = append(functions, string(k[len(prefix):]))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return functions, nil
}

func ExecuteFunction(functionName string, args ...interface{}) (interface{}, error) {
	log.Debug("Executing function", "functionName", functionName, "args", args)

	var code []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("function:" + functionName))
		if err != nil {
			log.Debug("Function not found in DB", "functionName", functionName, "error", err)
			return err
		}
		return item.Value(func(val []byte) error {
			code = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, FunctionError{Msg: fmt.Sprintf("Function %s not found: %v", functionName, err)}
	}

	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)

	_, err = i.Eval(string(code))
	if err != nil {
		log.Debug("Error evaluating function", "error", err)
		return nil, FunctionError{Msg: fmt.Sprintf("Error evaluating function: %v", err)}
	}

	v, err := i.Eval(functionName)
	if err != nil {
		log.Debug("Error getting function", "error", err)
		return nil, FunctionError{Msg: fmt.Sprintf("Error getting function: %v", err)}
	}

	fnType := reflect.TypeOf(v.Interface())
	if fnType.Kind() != reflect.Func {
		log.Debug("Not a function", "functionName", functionName)
		return nil, FunctionError{Msg: fmt.Sprintf("'%s' is not a function", functionName)}
	}

	if fnType.NumIn() != len(args) {
		log.Debug("Argument count mismatch", "expected", fnType.NumIn(), "got", len(args))
		return nil, FunctionError{Msg: fmt.Sprintf("Function expects %d arguments, but got %d", fnType.NumIn(), len(args))}
	}

	fnArgs := make([]reflect.Value, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		if reflect.TypeOf(args[i]) != fnType.In(i) {
			log.Debug("Argument type mismatch", "argIndex", i, "expected", fnType.In(i), "got", reflect.TypeOf(args[i]))
			return nil, FunctionError{Msg: fmt.Sprintf("Argument %d: expected %v, got %v", i+1, fnType.In(i), reflect.TypeOf(args[i]))}
		}
		fnArgs[i] = reflect.ValueOf(args[i])
	}

	results := reflect.ValueOf(v.Interface()).Call(fnArgs)

	log.Debug("Function executed", "functionName", functionName, "resultCount", len(results))

	if len(results) == 0 {
		return nil, nil
	} else if len(results) == 1 {
		return results[0].Interface(), nil
	} else {
		returnValues := make([]interface{}, len(results))
		for i, r := range results {
			returnValues[i] = r.Interface()
		}
		return returnValues, nil
	}
}

func extractImports(code string) ([]string, error) {
	re := regexp.MustCompile(`import\s+\(([\s\S]*?)\)`)
	matches := re.FindStringSubmatch(code)

	if len(matches) < 2 {
		return nil, nil
	}

	importBlock := matches[1]
	importLines := strings.Split(importBlock, "\n")

	var imports []string
	for _, line := range importLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		parts := strings.Fields(line)
		imp := strings.Trim(parts[len(parts)-1], "\"")
		imports = append(imports, imp)
	}

	return imports, nil
}

func args2interface(args []string) []interface{} {
	result := make([]interface{}, len(args))
	for i, v := range args {
		result[i] = v
	}
	return result
}

func generateFunctionWithSpinner(name, instruction string) func() error {
	return func() error {
		if name == "" || instruction == "" {
			return fmt.Errorf("name and instruction cannot be empty")
		}
		fmt.Printf("Generating function")
		err := GenerateFunction(name, instruction)
		if err != nil {
			return fmt.Errorf("error generating function: %v", err)
		}
		return nil
	}
}

func init() {
	log.SetLevel(map[bool]log.Level{true: log.DebugLevel, false: log.InfoLevel}[true])
	rootCmd.AddCommand(functionCmd)
	functionCmd.AddCommand(listFunctionsCmd)
	functionCmd.Flags().String("name", "", "The name of the function to generate or execute")
	functionCmd.Flags().String("instruction", "", "The description of the function to generate")
}

var listFunctionsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available functions",
	Long:  `Display a list of all functions that have been created`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Debug("Listing functions")
		functions, err := listFunctions()
		if err != nil {
			log.Debug("Error listing functions", "error", err)
			return fmt.Errorf("error listing functions: %v", err)
		}

		if len(functions) == 0 {
			log.Debug("No functions found")
			fmt.Println("No functions found.")
		} else {
			log.Debug("Functions found", "count", len(functions))
			fmt.Printf("Available functions (%d total):\n", len(functions))
			for _, f := range functions {
				fmt.Printf("- %s\n", f)
			}
		}
		return nil
	},
}

var functionCmd = &cobra.Command{
	Use:   "function",
	Short: "Handle Go functions",
	Long:  `Create and execute Go functions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		instruction, _ := cmd.Flags().GetString("instruction")

		log.Debug("Function command called", "name", name, "hasInstruction", instruction != "")

		if name == "" {
			log.Debug("Function name is required")
			return fmt.Errorf("function name is required")
		}

		if instruction != "" {
			log.Debug("Generating function", "name", name)
			p := tea.NewProgram(functionModel("Generating function...", generateFunctionWithSpinner(name, instruction)))
			_, err := p.Run()
			if err != nil {
				log.Debug("Error running TUI", "error", err)
				return fmt.Errorf("error running TUI: %v", err)
			}
		} else {
			log.Debug("Executing function", "name", name)
			fmt.Printf("Executing function '%s'...\n", name)
			result, err := ExecuteFunction(name, args2interface(args)...)
			if err != nil {
				log.Debug("Error executing function", "error", err)
				return fmt.Errorf("error executing function: %v", err)
			}
			log.Debug("Function execution completed", "result", result)
			fmt.Printf("Function execution completed. Result: %v\n", result)
		}
		return nil
	},
}
