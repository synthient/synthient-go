package synthient_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"time"

	"github.com/synthient/synthient-go"
)

func Example() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	info, err := client.LookupIP(context.Background(), netip.MustParseAddr("8.8.8.8"))
	if err != nil {
		log.Fatal(err)
	}
	if info.Network != nil {
		fmt.Println(info.Network.ISP)
	}
	if info.Intelligence != nil {
		fmt.Println(info.Intelligence.RiskScore, info.Intelligence.Categories)
	}
}

func ExampleNewClient() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
		synthient.WithMaxRetries(4),
		synthient.WithUserAgent("my-app/1.0"),
	)
	if err != nil {
		log.Fatal(err)
	}

	account, err := client.Account(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(account.Organization.Name, account.LookupQuota.Credits)
}

func ExampleClient_LookupIPs() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	addrs := []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("1.1.1.1")}
	results, err := client.LookupIPs(context.Background(), addrs)
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		if result.Intelligence == nil {
			fmt.Println(result.IP, "no intelligence available")
			continue
		}
		fmt.Println(result.IP, result.Intelligence.RiskScore)
	}
}

func ExampleWithResponseHook() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"),
		synthient.WithResponseHook(func(_ context.Context, meta synthient.ResponseMetadata) {
			if meta.RateLimit != nil {
				fmt.Println(meta.StatusCode, meta.RateLimit.Remaining, meta.RateLimit.Reset)
			}
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Account(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_LookupDomain() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	domain, err := client.LookupDomain(context.Background(), "example.com", synthient.LookupDomainParams{Limit: 10})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(domain.Status, domain.Stats.Events24h)
	for _, event := range domain.RecentEvents {
		fmt.Println(event.Timestamp.Time(), event.TargetSubdomain, event.Port)
	}
}

func ExampleAPIError() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.LookupIP(context.Background(), netip.MustParseAddr("8.8.8.8"))
	apiErr, ok := errors.AsType[*synthient.APIError](err)
	if !ok {
		log.Fatal(err)
	}
	switch {
	case apiErr.StatusCode == http.StatusPaymentRequired:
		fmt.Println("out of lookup credits")
	case apiErr.StatusCode == http.StatusTooManyRequests && apiErr.RetryAfter > 0:
		fmt.Println("rate limited, retry after", apiErr.RetryAfter)
	case apiErr.StatusCode == http.StatusTooManyRequests:
		fmt.Println("monthly request budget spent")
	default:
		fmt.Println("request", apiErr.RequestID, "failed:", cmp.Or(apiErr.Detail, apiErr.Title))
	}
}

func ExampleFeed_Stream() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	for ctx.Err() == nil {
		for event, err := range client.Proxies.Stream(ctx) {
			if err != nil {
				apiErr, ok := errors.AsType[*synthient.APIError](err)
				if ok && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
					log.Print(err)
					return
				}
				log.Printf("proxy stream: %v", err)
				// Avoid reconnecting in a tight loop.
				time.Sleep(2 * time.Second)
				break
			}
			fmt.Println(event.IP, event.Provider, event.Type)
		}
		// Reconnect after the server closes the stream.
	}
}

func ExampleFeed_AllExports() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	for export, err := range client.Proxies.AllExports(context.Background()) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(export.ID, export.Kind, export.SizeBytes)
	}
}
