package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		panic(err)
	}

	hash, err := contract.CanonicalJSONHash(value)
	if err != nil {
		panic(err)
	}

	fmt.Print(hash)
}
