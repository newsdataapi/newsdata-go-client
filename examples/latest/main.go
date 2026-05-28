// A minimal example: fetch the latest news with a few filters.
//
//	NEWSDATA_API_KEY=<your key> go run ./examples/latest
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/newsdataapi/newsdata-go-client"
)

func main() {
	apiKey := os.Getenv("NEWSDATA_API_KEY")
	if apiKey == "" {
		log.Fatal("set NEWSDATA_API_KEY in your env")
	}

	client, err := newsdataapi.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	// Values may be a single string or a []string (sent comma-joined).
	resp, err := client.Latest(context.Background(), newsdataapi.Params{
		"q":        "bitcoin",
		"country":  []string{"us", "gb"},
		"language": "en",
	})
	if err != nil {
		log.Fatal(err)
	}

	articles, err := resp.Articles()
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range articles {
		fmt.Printf("- %s\n  %s\n", a.Title, a.Link)
	}
}
