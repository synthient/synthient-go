// Command lookup_domain prints the honeypot traffic Synthient has observed for
// a domain. Each lookup costs one credit.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/lookup_domain example.com
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/synthient/synthient-go"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: lookup_domain <domain>")
	}

	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	domain, err := client.LookupDomain(context.Background(), os.Args[1], synthient.LookupDomainParams{Limit: 5})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("domain:", domain.Domain, domain.Status)
	fmt.Printf("events: %d in 24h, %d in 30d\n", domain.Stats.Events24h, domain.Stats.TotalEvents30d)
	fmt.Printf("unique IPs: %d in 24h, %d in 30d\n", domain.UniqueIPs.Value24h, domain.UniqueIPs.Value30d)
	for _, subdomain := range domain.TopSubdomains {
		fmt.Printf("subdomain: %s (%d events)\n", subdomain.Subdomain, subdomain.Count)
	}
	for _, event := range domain.RecentEvents {
		at := event.Timestamp.Time().Format(time.DateTime)
		fmt.Printf("event: %s %s -> %s:%d\n", at, event.SourceIP, event.TargetSubdomain, event.Port)
	}
}
