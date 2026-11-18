// Command download_dataset downloads the newest day of the Geolocation
// dataset as a MaxMind DB to geolocation.mmdb and verifies its checksum. It
// needs an API key with the GEOLOCATION_FEED scope.
//
//	SYNTHIENT_API_KEY=... go run ./_examples/download_dataset
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
	"slices"

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
	page, err := client.Geolocation.ListExports(ctx, synthient.ListDatasetExportsParams{Limit: 1})
	if err != nil {
		return err
	}
	if len(page.Exports) == 0 {
		return errors.New("no geolocation export found")
	}
	export := page.Exports[0]
	i := slices.IndexFunc(export.Formats, func(f synthient.ExportFile) bool {
		return f.Format == synthient.ExportFormatMMDB
	})
	if i < 0 {
		return fmt.Errorf("export %s has no mmdb file", export.ID)
	}

	params := synthient.DatasetExportParams{Format: synthient.ExportFormatMMDB}
	body, err := client.Geolocation.Download(ctx, export.ID, params)
	if err != nil {
		return err
	}
	defer body.Close()

	file, err := os.Create("geolocation.mmdb")
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(file, hash), body)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}

	if hex.EncodeToString(hash.Sum(nil)) != export.Formats[i].Checksum {
		return errors.New("checksum mismatch")
	}
	fmt.Println("saved", export.ID, "to geolocation.mmdb")
	return nil
}
