package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/integrations/productsource"
)

func main() {
	if len(os.Args) < 2 || strings.TrimSpace(os.Args[1]) == "" {
		fmt.Fprintln(os.Stderr, "usage: cookies-resolve-product <douyin-product-link-or-share-message>")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	snapshot, err := productsource.NewResolver().Resolve(ctx, strings.Join(os.Args[1:], " "))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}
