// Command stream prints proxy sightings from the real-time proxies feed until
// interrupted, reconnecting whenever the server closes the stream. It stops
// on a rejected key or a missing scope, and needs an API key with the
// PROXY_FIREHOSE scope.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/stream
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/synthient/synthient-go"
)

func main() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err = run(ctx, client)
	stop()
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, client *synthient.Client) error {
	for ctx.Err() == nil {
		for event, err := range client.Proxies.Stream(ctx) {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			if err != nil {
				apiErr, ok := errors.AsType[*synthient.APIError](err)
				if ok && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
					return err
				}
				log.Printf("stream: %v", err)
				// Stream connections are limited to 30 per minute per team.
				select {
				case <-ctx.Done():
				case <-time.After(2 * time.Second):
				}
				break
			}
			at := event.Timestamp.Time().Format(time.RFC3339)
			fmt.Println(at, event.IP, event.Provider, event.Type, event.CountryCode)
		}
		// Reconnect after the server closes the stream.
	}
	return nil
}
