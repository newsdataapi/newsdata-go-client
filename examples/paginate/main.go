// Two flavours of pagination:
//
//   - ScrollAll follows nextPage cursors and returns one merged response,
//     capped at maxResults.
//   - Paginate calls a callback once per page so you can stream-process.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/newsdataapi/newsdata-go-client"
)

func main() {
	client, err := newsdataapi.NewClient(os.Getenv("NEWSDATA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// 1. ScrollAll: merge everything into a single Response (cap at 200 articles).
	merged, err := client.ScrollAll(ctx, newsdataapi.EndpointLatest,
		newsdataapi.Params{"q": "news"}, 200)
	if err != nil {
		log.Fatal(err)
	}
	mergedArticles, _ := merged.Articles()
	fmt.Println("merged total:", len(mergedArticles))

	// 2. Paginate: process one page at a time. Return false from the callback
	// to stop early.
	page := 0
	err = client.Paginate(ctx, newsdataapi.EndpointLatest,
		newsdataapi.Params{"q": "news"},
		func(r *newsdataapi.Response, e error) bool {
			if e != nil {
				log.Println(e)
				return false
			}
			page++
			arts, _ := r.Articles()
			fmt.Printf("page %d: %d articles\n", page, len(arts))
			return page < 5 // stop after 5 pages
		})
	if err != nil {
		log.Fatal(err)
	}
}
