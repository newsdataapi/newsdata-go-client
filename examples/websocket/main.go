// Real-time news streaming.
//
//	NEWSDATA_API_KEY=<your key> go run ./examples/websocket
//
// Articles are matched by a registered query. If NEWSDATA_REGISTRATION_ID is
// set, that query is streamed directly; otherwise the example registers a demo
// query (q="pizza") first and prints the resulting registration_id so you can
// reuse it on the next run — or remove it later with WebSocketDelete.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	// Stop cleanly on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registrationID := os.Getenv("NEWSDATA_REGISTRATION_ID")
	if registrationID == "" {
		if registrationID, err = registerDemoQuery(ctx, client); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("streaming %s — Ctrl-C to stop\n", registrationID)

	ws := newsdataapi.NewWebSocket(client)
	err = ws.Stream(ctx, registrationID, func(resp *newsdataapi.Response) error {
		articles, err := resp.Articles()
		if err != nil {
			return err
		}
		for _, a := range articles {
			fmt.Printf("%s - %s\n", a.Title, a.Link)
		}
		return nil
	})

	var authErr *newsdataapi.NewsdataWebSocketAuthError
	switch {
	case err == nil, errors.Is(err, context.Canceled):
		fmt.Println("stopped")
	case errors.As(err, &authErr):
		log.Fatalf("rejected: %v", err)
	default:
		log.Fatalf("stream error: %v", err)
	}
}

// registerDemoQuery registers q="pizza" and returns its registration_id.
// Registering an identical query again answers HTTP 409 with the existing id
// in the response body — reuse it instead of failing.
func registerDemoQuery(ctx context.Context, c *newsdataapi.Client) (string, error) {
	resp, err := c.WebSocketRegister(ctx, newsdataapi.Params{"q": "pizza"})
	if err != nil {
		var apiErr *newsdataapi.NewsdataAPIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
			if id := registrationIDFrom(apiErr.ResponseBody); id != "" {
				fmt.Printf("query already registered; reusing %s\n", id)
				return id, nil
			}
		}
		return "", err
	}
	agg, err := resp.Aggregate()
	if err != nil {
		return "", err
	}
	id, _ := agg["registration_id"].(string)
	fmt.Printf("registered demo query q=\"pizza\" -> %s\n", id)
	return id, nil
}

func registrationIDFrom(body []byte) string {
	var parsed struct {
		Results struct {
			RegistrationID string `json:"registration_id"`
		} `json:"results"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return ""
	}
	return parsed.Results.RegistrationID
}
