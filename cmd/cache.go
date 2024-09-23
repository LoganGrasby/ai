package cmd

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/spf13/viper"
	usearch "github.com/unum-cloud/usearch/golang"
)

var (
	index       *usearch.Index
	db          *badger.DB
	keyToUint64 map[string]uint64
	uint64ToKey map[uint64]string
	apiKey      string
)

func computeVector(value string) []float32 {
	provider := viper.GetString("provider")
	model := viper.GetString("embedding_model")
	if model == "" {
		if provider == "OpenAI" {
			model = "text-embedding-3-small"
		} else if provider == "Ollama" {
			model = "bge-m3"
		}
	} else {
		fmt.Printf("No default embedding model specified for provider %s\n", provider)
		os.Exit(1)
	}
	apiKey := getAPIKey(provider)
	if apiKey == "" {
		fmt.Printf("Error: API key not set for provider %s. Use 'ai config' or set the appropriate environment variable.\n", provider)
		os.Exit(1)
	}
	embeddings, err := callAPI[[]float32](provider, model, apiKey, value, EmbeddingsRequest)
	if err != nil {
		fmt.Printf("Error calling embeddings API: %v\n", err)
		os.Exit(1)
	}
	return embeddings
}

func stringToUint64(s string) (uint64, error) {
	b := []byte(s)
	if len(b) != 8 {
		return 0, fmt.Errorf("input string must be exactly 8 bytes long")
	}
	return binary.BigEndian.Uint64(b), nil
}

func addToVecDB(vector []float32, key string, value string) error {
	if index == nil {
		panic("Vector index not initialized")
	}
	uintKey := hashString(key)
	err := index.Reserve(uint(1))
	if err != nil {
		return fmt.Errorf("failed to reserve space in index: %v", err)
	}

	err = index.Add(uintKey, vector)
	if err != nil {
		return fmt.Errorf("failed to add vector to index: %v", err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set(uint64ToBytes(uintKey), []byte(value))
	})
	if err != nil {
		return fmt.Errorf("failed to store value in BadgerDB: %v", err)
	}

	err = index.Save(indexFile)
	if err != nil {
		return fmt.Errorf("failed to save index: %v", err)
	}
	return nil
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func getFromDB(key []byte) (string, error) {
	var valCopy []byte
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		valCopy, err = item.ValueCopy(nil)
		return err
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return "", fmt.Errorf("key not found: %s", key)
		}
		return "", fmt.Errorf("error retrieving value from BadgerDB: %v", err)
	}
	return string(valCopy), nil
}

func getCachedResponse(textCommand string) (string, bool, []float32) {
	if index == nil {
		panic("Vector index is unavailable")
	}
	vector := computeVector(textCommand)
	keys, distances, err := index.Search(vector, uint(k))
	if err != nil {
		panic(fmt.Sprintf("Failed to search Index: %v", err))
	}
	if len(keys) == 0 {
		return "", false, vector
	}
	key := keys[0]
	distance := distances[0]
	maxDistance := viper.GetFloat64("max_distance")
	if maxDistance == 0 {
		maxDistance = defaultMaxDistance
	}
	if float64(distance) > maxDistance {
		return "", false, vector
	}
	value, err := getFromDB(uint64ToBytes(key))
	if err != nil {
		fmt.Println("Failed to retrieve value from DB:", err)
		return "", false, vector
	}
	return value, true, vector
}

func uint64ToBytes(i uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, i)
	return buf
}
