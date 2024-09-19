package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"

	"github.com/spf13/viper"
	usearch "github.com/unum-cloud/usearch/golang"
)

func loadCache() {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			cache = make(map[string]string)
			keyToUint64 = make(map[string]uint64)
			uint64ToKey = make(map[uint64]string)
			initIndex()
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

	keyToUint64 = make(map[string]uint64)
	uint64ToKey = make(map[uint64]string)
	for key := range cache {
		uintKey := hashStringToUint64(key)
		keyToUint64[key] = uintKey
		uint64ToKey[uintKey] = key
	}
	initIndex()
}

func saveCache() {
	data, err := json.Marshal(cache)
	if err != nil {
		fmt.Println("Error serializing cache:", err)
		return
	}
	err = os.WriteFile(cacheFile, data, 0644)
	if err != nil {
		fmt.Println("Error writing cache file:", err)
	}

	err = index.Save(indexFile)
	if err != nil {
		fmt.Println("Error saving index file:", err)
	}
}

func initIndex() {
	conf := usearch.DefaultConfig(uint(vectorSize))
	var err error
	index, err = usearch.NewIndex(conf)
	if err != nil {
		fmt.Println("Failed to create Index:", err)
		os.Exit(1)
	}

	if err != nil {
		fmt.Println("Failed to reserve space in Index:", err)
		os.Exit(1)
	}

	err = index.Load(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			err = index.Reserve(uint(len(cache)))
			for key := range cache {
				uintKey := keyToUint64[key]
				vector := computeVector(key)
				err = index.Add(uintKey, vector)
				if err != nil {
					fmt.Println("Failed to add vector to index:", err)
					os.Exit(1)
				}
			}
			err = index.Save(indexFile)
			if err != nil {
				fmt.Println("Failed to save index:", err)
				os.Exit(1)
			} else {
				index, err := usearch.NewIndex(conf)
				index.Save(indexFile)
				if err != nil {
					panic("Failed to save index")
				}
				if err != nil {
					panic("Failed to create Index")
				}
			}
		}
	}
}

func computeVector(value string) []float32 {
	provider := viper.GetString("provider")
	model := viper.GetString("embedding_model")
	if model == "" {
		if provider == "OpenAI" {
			model = "text-embedding-3-small"
		} else {
			fmt.Printf("No default embedding model specified for provider %s\n", provider)
			os.Exit(1)
		}
	}
	apiKey := getAPIKey(provider)
	if apiKey == "" {
		fmt.Printf("Error: API key not set for provider %s. Use 'ai config' or set the appropriate environment variable.\n", provider)
		os.Exit(1)
	}

	var embeddings []float32
	var err error
	embeddings, err = callAPI[[]float32](provider, model, apiKey, value, EmbeddingsRequest)
	if err != nil {
		fmt.Printf("Error calling embeddings API: %v\n", err)
		os.Exit(1)
	}
	return embeddings
}

func hashStringToUint64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func addToVecDB(key string, value string) {

	cache[key] = value

	vector := computeVector(key)

	uintKey := hashStringToUint64(key)

	keyToUint64[key] = uintKey
	uint64ToKey[uintKey] = key

	err := index.Reserve(uint(1))
	err = index.Add(uintKey, vector)
	if err != nil {
		fmt.Println("Failed to add vector to index:", err)
	}
	saveCache()
}

func removeFromCache(key string) {
	delete(cache, key)

	uintKey := keyToUint64[key]

	delete(keyToUint64, key)
	delete(uint64ToKey, uintKey)

	err := index.Remove(uintKey)
	if err != nil {
		fmt.Println("Failed to remove vector from index:", err)
	}

	saveCache()
}

func getCachedResponse(textCommand string) (string, string, bool) {
	vector := computeVector(textCommand)
	keys, distances, err := index.Search(vector, uint(k))
	if err != nil {
		fmt.Println("Failed to search index:", err)
		return "", "", false
	}
	if len(keys) == 0 {
		return "", "", false
	}
	uintKey := keys[0]
	distance := distances[0]
	maxDistance := viper.GetFloat64("max_distance")
	if maxDistance == 0 {
		maxDistance = defaultMaxDistance
	}
	if float64(distance) > maxDistance {
		return "", "", false
	}
	key := uint64ToKey[uintKey]
	cachedResponse, exists := cache[key]
	if !exists {
		return "", "", false
	}
	return cachedResponse, key, true
}
