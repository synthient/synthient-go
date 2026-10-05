// Command account prints the account that owns the API key, its scopes, and
// its remaining lookup credits.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/account
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
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	account, err := client.Account(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	resetsIn := time.Duration(account.LookupQuota.ResetsIn) * time.Second
	fmt.Printf("%s %s <%s>\n", account.FirstName, account.LastName, account.Email)
	fmt.Println("organization:", account.Organization.Name)
	fmt.Println("scopes:", account.Scopes)
	fmt.Printf("credits: %d (resets in %s)\n", account.LookupQuota.Credits, resetsIn)
}
