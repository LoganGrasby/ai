package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

func loadCache() {
	data, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			cache = make(map[string]string)
			return
		}
		fmt.Println("Error reading cache file:", err)
		os.Exit(1)
	}
	err = json.Unmarshal(data, &cache)
	if err != nil {
		fmt.Println("Error parsing cache file:", err)
		os.Exit(1)
	}
}

func saveCache() {
	data, err := json.Marshal(cache)
	if err != nil {
		fmt.Println("Error serializing cache:", err)
		return
	}
	err = ioutil.WriteFile(cacheFile, data, 0644)
	if err != nil {
		fmt.Println("Error writing cache file:", err)
	}
}
