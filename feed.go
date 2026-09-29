package synthient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Feed is a Synthient data feed. Each feed has a real-time stream of events
// of type T and hourly parquet exports that roll up into daily exports.
// Access feeds through the fields of Client, such as Client.Proxies.
type Feed[T any] struct {
	client *Client
	path   string
}

// ExportID identifies a parquet export: LatestExport for the most recent
// hourly export, a UTC day as "YYYY-MM-DD", or a UTC hour as
// "YYYY-MM-DD/HH". Export.ID values can be passed as they are.
type ExportID string

// LatestExport identifies a feed's most recent hourly export.
const LatestExport ExportID = "latest"

const hourlyLayout = "2006-01-02/15"

// DailyExport returns the ID of the daily export for the UTC day containing t.
func DailyExport(t time.Time) ExportID {
	return ExportID(t.UTC().Format(time.DateOnly))
}

// HourlyExport returns the ID of the hourly export for the UTC hour
// containing t.
func HourlyExport(t time.Time) ExportID {
	return ExportID(t.UTC().Format(hourlyLayout))
}

func (id ExportID) path() (string, error) {
	if id == LatestExport {
		return "latest", nil
	}
	day, err := time.Parse(time.DateOnly, string(id))
	if err == nil {
		return day.Format(time.DateOnly), nil
	}
	hour, err := time.Parse(hourlyLayout, string(id))
	if err == nil {
		return hour.Format(time.DateOnly) + "/" + strconv.Itoa(hour.Hour()), nil
	}
	return "", fmt.Errorf("synthient: invalid export ID %q", string(id))
}

// ListExportsParams holds optional parameters for listing exports.
type ListExportsParams struct {
	// Limit is the page size, from 1 to 500. Zero uses the API default of 100.
	Limit int
	// Cursor continues a listing from a previous page's NextCursor.
	Cursor string
}

// ExportPage is one page of a feed's exports, newest first.
type ExportPage struct {
	// Stream is the feed's stream identifier, such as "proxies".
	Stream string `json:"stream"`
	// Exports are the exports on this page.
	Exports []Export `json:"feeds"`
	// NextCursor fetches the next page. It is empty on the last page.
	NextCursor string `json:"next_cursor"`
}

// Export describes a parquet export of a feed.
type Export struct {
	// ID identifies the export for ExportMeta, Download, and DownloadURL.
	ID ExportID `json:"id"`
	// Kind is "hourly" or "daily".
	Kind string `json:"kind"`
	// Date is the UTC day of the export as "YYYY-MM-DD".
	Date string `json:"date"`
	// Hour is the UTC hour of an hourly export, or nil for a daily export.
	Hour *int `json:"hour"`
	// SizeBytes is the size of the parquet file in bytes.
	SizeBytes int64 `json:"size_bytes"`
	// RowCount is the number of rows in the parquet file.
	RowCount int64 `json:"row_count"`
	// Checksum is the hex-encoded SHA-256 of the parquet file.
	Checksum string `json:"checksum"`
	// CreatedAt is when the export was produced.
	CreatedAt Timestamp `json:"created_at"`
	// DownloadPath is the API path of the export's download, including the
	// version prefix.
	DownloadPath string `json:"download_path"`
}

// ExportMeta describes a parquet export without its contents.
type ExportMeta struct {
	// ID identifies the export.
	ID ExportID `json:"id"`
	// Stream is the feed's stream identifier.
	Stream string `json:"stream"`
	// Kind is "hourly" or "daily".
	Kind string `json:"kind"`
	// Format is the file format, "parquet".
	Format string `json:"format"`
	// Date is the start of the export's UTC day or hour.
	Date Timestamp `json:"date"`
	// Hour is the UTC hour of an hourly export, or nil for a daily export.
	Hour *int `json:"hour"`
	// Rows is the number of rows in the parquet file.
	Rows int64 `json:"rows"`
	// Size is the size of the parquet file in bytes.
	Size int64 `json:"size"`
	// Checksum is the hex-encoded SHA-256 of the parquet file.
	Checksum string `json:"checksum"`
	// CreatedAt is when the export was produced.
	CreatedAt Timestamp `json:"created_at"`
	// Schema lists the parquet columns.
	Schema ParquetSchema `json:"schema"`
}

// ParquetSchema lists the columns of a parquet export.
type ParquetSchema struct {
	// Fields are the columns in file order.
	Fields []ParquetColumn `json:"fields"`
}

// ParquetColumn is a column of a parquet export.
type ParquetColumn struct {
	// Name is the column name.
	Name string `json:"name"`
	// Type is the column type, such as "string" or "int64".
	Type string `json:"type"`
}

// ListExports returns one page of the feed's exports, newest first. params
// may be nil. Use AllExports to iterate over every page.
func (f *Feed[T]) ListExports(ctx context.Context, params *ListExportsParams) (*ExportPage, error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Cursor != "" {
			query.Set("cursor", params.Cursor)
		}
	}

	var page ExportPage
	err := f.client.doJSON(
		ctx,
		request{method: http.MethodGet, path: f.path + "/export", query: query},
		&page,
	)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// AllExports returns an iterator over the feed's exports, newest first. It
// fetches pages lazily as iteration proceeds, and stops without fetching
// more when the loop breaks. An error is yielded once and ends the
// iteration. params may be nil and is not modified.
func (f *Feed[T]) AllExports(
	ctx context.Context,
	params *ListExportsParams,
) iter.Seq2[Export, error] {
	var start ListExportsParams
	if params != nil {
		start = *params
	}

	return func(yield func(Export, error) bool) {
		page := start
		seen := map[string]bool{page.Cursor: true}
		for {
			result, err := f.ListExports(ctx, &page)
			if err != nil {
				yield(Export{}, err)
				return
			}
			for _, export := range result.Exports {
				if !yield(export, nil) {
					return
				}
			}
			if result.NextCursor == "" {
				return
			}
			if seen[result.NextCursor] {
				yield(Export{}, errors.New("synthient: export listing returned a repeated cursor"))
				return
			}
			seen[result.NextCursor] = true
			page.Cursor = result.NextCursor
		}
	}
}

// ExportMeta returns the checksum, size, row count, and parquet schema of an
// export.
func (f *Feed[T]) ExportMeta(ctx context.Context, id ExportID) (*ExportMeta, error) {
	path, err := id.path()
	if err != nil {
		return nil, err
	}

	var meta ExportMeta
	err = f.client.doJSON(
		ctx,
		request{method: http.MethodGet, path: f.path + "/export/" + path + "/meta"},
		&meta,
	)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// DownloadURL returns a presigned URL for an export's parquet file. The URL
// is valid for 24 hours and grants access without an API key, so treat it as
// a secret. It is useful for tools that read parquet over HTTPS directly.
func (f *Feed[T]) DownloadURL(ctx context.Context, id ExportID) (*url.URL, error) {
	path, err := id.path()
	if err != nil {
		return nil, err
	}

	resp, err := f.client.send(
		ctx,
		request{method: http.MethodGet, path: f.path + "/export/" + path},
	)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, newAPIError(resp)
	}
	if resp.StatusCode < http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"synthient: expected a redirect to the export, got status %d",
			resp.StatusCode,
		)
	}
	location, err := resp.Location()
	if err != nil {
		return nil, errors.New("synthient: export redirect has no valid location")
	}
	if location.Scheme != "https" && location.Scheme != f.client.baseURL.Scheme {
		return nil, errors.New("synthient: export redirect does not use https")
	}
	return location, nil
}

// Download returns the contents of an export's parquet file. The caller must
// close the returned reader. The API key is sent only to the Synthient API,
// never to the storage host that serves the file, and errors never include
// the presigned URL. A non-success response from either returns an
// *APIError.
func (f *Feed[T]) Download(ctx context.Context, id ExportID) (io.ReadCloser, error) {
	location, err := f.DownloadURL(ctx, id)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), http.NoBody)
	if err != nil {
		return nil, errors.New("synthient: invalid export download URL")
	}
	req.Header.Set("User-Agent", f.client.userAgent)

	resp, err := f.client.storage.Do(req)
	if err != nil {
		urlErr, ok := errors.AsType[*url.Error](err)
		if ok {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("synthient: download export: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer closeBody(resp)
		return nil, newAPIError(resp)
	}
	return resp.Body, nil
}

func storageRedirectPolicy(
	check func(*http.Request, []*http.Request) error,
) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if check != nil {
			err := check(req, via)
			if err != nil {
				return err
			}
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
			return errors.New("synthient: export redirect has an unsupported scheme")
		}
		for _, previous := range via {
			if previous.URL.Scheme == "https" && req.URL.Scheme != "https" {
				return errors.New("synthient: export redirect does not use https")
			}
		}
		if len(via) >= 10 {
			return errors.New("synthient: too many export redirects")
		}
		return nil
	}
}
