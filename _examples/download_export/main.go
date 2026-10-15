// Command download_export lists the newest exports of the proxies feed, then
// downloads the most recent daily export to proxies.parquet and verifies its
// checksum. It needs an API key with the PROXY_FEEDS scope.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/download_export
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/synthient/synthient-go"
)

func main() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	err = run(context.Background(), client)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, client *synthient.Client) error {
	var daily *synthient.Export
	for export, err := range client.Proxies.AllExports(ctx) {
		if err != nil {
			return err
		}
		fmt.Printf("%-16s %-6s %10d rows %12d bytes\n", export.ID, export.Kind, export.RowCount, export.SizeBytes)
		if export.Kind == synthient.ExportKindDaily {
			daily = &export
			break
		}
	}
	if daily == nil {
		return errors.New("no daily export found")
	}

	body, err := client.Proxies.Download(ctx, daily.ID)
	if err != nil {
		return err
	}
	defer body.Close()

	file, err := os.Create("proxies.parquet")
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(file, hash), body)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}

	if hex.EncodeToString(hash.Sum(nil)) != daily.Checksum {
		return errors.New("checksum mismatch")
	}
	fmt.Println("saved", daily.ID, "to proxies.parquet")
	return nil
}
