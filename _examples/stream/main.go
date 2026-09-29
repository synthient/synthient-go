// Command stream prints proxy sightings from the real-time proxies feed until
// interrupted, reconnecting whenever the server closes the stream. It needs
// an API key with the PROXY_FIREHOSE scope.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/stream
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	defer stop()

	for ctx.Err() == nil {
		for event, err := range client.Proxies.Stream(ctx) {
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				log.Printf("stream: %v", err)
				break
			}
			at := event.Timestamp.Time().Format(time.RFC3339)
			fmt.Println(at, event.IP, event.Provider, event.Type, event.CountryCode)
		}

		// The server closes each stream after about 30 minutes. Pause before
		// reconnecting so repeated failures stay under the connection rate
		// limit.
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}
