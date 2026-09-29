// Command lookup_ip prints what Synthient knows about an IP address. Each
// lookup costs one credit.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/lookup_ip 8.8.8.8
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/synthient/synthient-go"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: lookup_ip <ip>")
	}
	addr, err := netip.ParseAddr(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	info, err := client.LookupIP(context.Background(), addr)
	if err != nil {
		apiErr, ok := errors.AsType[*synthient.APIError](err)
		if ok && apiErr.StatusCode == http.StatusPaymentRequired {
			log.Fatal("out of lookup credits")
		}
		log.Fatal(err)
	}

	fmt.Println("ip:", info.IP)
	if info.Network != nil {
		fmt.Printf("network: %s (AS%d, %s)\n", info.Network.ISP, info.Network.ASN, info.Network.Type)
	}
	if info.Location != nil {
		fmt.Printf("location: %s, %s\n", info.Location.City, info.Location.Country)
	}
	if info.Intelligence == nil {
		fmt.Println("no intelligence available")
		return
	}
	fmt.Println("risk score:", info.Intelligence.RiskScore)
	fmt.Println("categories:", strings.Join(info.Intelligence.Categories, ", "))
	for _, provider := range info.Intelligence.Providers {
		lastSeen := provider.LastSeen.Time().Format(time.DateTime)
		fmt.Printf("provider: %s (%s), last seen %s\n", provider.Name, provider.Type, lastSeen)
	}
}
