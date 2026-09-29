// Command lookup_ips looks up several IP addresses in one batch request and
// prints the risk score and categories of each.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/lookup_ips 8.8.8.8 1.1.1.1
package main

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"

	"github.com/synthient/synthient-go"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: lookup_ips <ip>...")
	}
	addrs := make([]netip.Addr, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		addr, err := netip.ParseAddr(arg)
		if err != nil {
			log.Fatal(err)
		}
		addrs = append(addrs, addr)
	}

	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	results, err := client.LookupIPs(context.Background(), addrs)
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		if result.Intelligence == nil {
			fmt.Printf("%-39s no intelligence available\n", result.IP)
			continue
		}
		categories := strings.Join(result.Intelligence.Categories, ", ")
		fmt.Printf("%-39s risk %3d  %s\n", result.IP, result.Intelligence.RiskScore, categories)
	}
}
