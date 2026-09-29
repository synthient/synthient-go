package synthient_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/synthient/synthient-go"
)

func ExampleFeed_Download() {
	client, err := synthient.NewClient(os.Getenv("SYNTHIENT_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	yesterday := synthient.DailyExport(time.Now().AddDate(0, 0, -1))
	err = saveExport(context.Background(), client.Proxies, yesterday, "proxies.parquet")
	if err != nil {
		log.Fatal(err)
	}
}

// saveExport writes an export to path and verifies it against the export's
// SHA-256 checksum.
func saveExport(ctx context.Context, feed *synthient.Feed[synthient.ProxyEvent], id synthient.ExportID, path string) error {
	meta, err := feed.ExportMeta(ctx, id)
	if err != nil {
		return err
	}
	body, err := feed.Download(ctx, id)
	if err != nil {
		return err
	}
	defer body.Close()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(file, hash), body)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}

	if hex.EncodeToString(hash.Sum(nil)) != meta.Checksum {
		return errors.New("checksum mismatch")
	}
	return nil
}
