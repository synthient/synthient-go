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

// Feed is a Synthient data feed: a real-time stream of events of type T and
// hourly parquet exports that roll up into daily exports. Access feeds
// through the fields of Client, such as Client.Proxies.
//
// An export ID is LatestExport, a UTC day as "YYYY-MM-DD", or a UTC hour as
// "YYYY-MM-DD/H", where a zero-padded hour is also accepted. Export.ID values
// can be passed as they are.
//
// An hourly export is available until its day's daily export is rolled up,
// shortly after 00:30 UTC the next day. After that, requests for the hour
// return a 400 *APIError. While a daily export is being rolled up, requests
// for it return a 404 *APIError with RetryAfter set.
type Feed[T any] struct {
	client *Client
	path   string
}

// LatestExport identifies a feed's freshest export: the newest hourly export
// during the day, or the daily export once that day's hours are rolled up.
const LatestExport = "latest"

const hourlyLayout = "2006-01-02/15"

// DailyExport returns the ID of the daily export for the UTC day containing t,
// which is also the form ListExportsParams.Day takes.
func DailyExport(t time.Time) string {
	return t.UTC().Format(time.DateOnly)
}

// HourlyExport returns the ID of the hourly export for the UTC hour
// containing t.
func HourlyExport(t time.Time) string {
	return t.UTC().Format(hourlyLayout)
}

func exportPath(id string) (string, error) {
	if id == LatestExport {
		return id, nil
	}
	day, err := time.Parse(time.DateOnly, id)
	if err == nil {
		return day.Format(time.DateOnly), nil
	}
	hour, err := time.Parse(hourlyLayout, id)
	if err == nil {
		return hour.Format(time.DateOnly) + "/" + strconv.Itoa(hour.Hour()), nil
	}
	return "", fmt.Errorf("synthient: invalid export ID %q", id)
}

// ListExportsParams holds optional parameters for listing a feed's exports.
type ListExportsParams struct {
	// Limit is the page size, from 1 to 500. Zero uses the API default of 100.
	Limit int
	// Cursor continues a listing from a previous page's NextCursor.
	Cursor string
	// Day lists only the exports of a UTC day, as "YYYY-MM-DD".
	Day string
	// Variant lists the lite exports when set to ExportVariantLite. A lite
	// export has the same ID as the full one, so pass the variant again to
	// ExportMeta, DownloadURL, and Download.
	Variant ExportVariant
}

// ExportParams holds optional parameters for a feed export.
type ExportParams struct {
	// Variant selects the export's rendering. Empty uses the API default,
	// ExportVariantFull.
	Variant ExportVariant
}

// ExportVariant is the rendering of a feed export. Only Proxies,
// Anonymizers, and JA4T publish lite exports. On other feeds, a lite listing
// is empty and other lite requests return a 404 *APIError.
type ExportVariant string

// Export variants.
const (
	// ExportVariantFull has every sighting.
	ExportVariantFull ExportVariant = "full"
	// ExportVariantLite keeps the latest row per key in the export's hour or
	// day, such as one row per address and provider for Proxies. It is
	// published shortly after the full export, and requests for it return a
	// 404 *APIError until then.
	ExportVariantLite ExportVariant = "lite"
)

// ExportVolumeParams holds optional parameters for ExportVolume.
type ExportVolumeParams struct {
	// Days is the window in UTC days, including today, from 1 to 366. Zero
	// uses the API default of 90.
	Days int
}

// ExportPage is one page of a feed's or dataset's exports, newest first.
type ExportPage struct {
	// Stream is the stream identifier, such as "proxies".
	Stream  string   `json:"stream"`
	Exports []Export `json:"feeds"`
	// NextCursor fetches the next page. It is empty on the last page.
	NextCursor string `json:"next_cursor"`
}

// Export describes an export of a feed or dataset.
type Export struct {
	// ID identifies the export for ExportMeta, Download, and DownloadURL. On
	// the first page of a feed, the newest hourly export may have the ID
	// LatestExport.
	ID   string     `json:"id"`
	Kind ExportKind `json:"kind"`
	// Date is the UTC day of the export as "YYYY-MM-DD".
	Date string `json:"date"`
	// Hour is the UTC hour of an hourly export, or nil for a daily export.
	Hour *int `json:"hour"`
	// SizeBytes is the size of the first file in Formats in bytes.
	SizeBytes int64 `json:"size_bytes"`
	RowCount  int64 `json:"row_count"`
	// Checksum is the hex-encoded SHA-256 of the first file in Formats.
	Checksum  string    `json:"checksum"`
	CreatedAt Timestamp `json:"created_at"`
	// DownloadPath is the API path of the export's download, including the
	// version prefix.
	DownloadPath string `json:"download_path"`
	// Formats lists every file of the export, its own format first.
	Formats []ExportFile `json:"formats"`
	// Entities lists the parquet files inside a WHOIS archive, without their
	// schemas. RowCount is their sum. It is empty for other exports.
	Entities []ExportEntity `json:"entities"`
	// Variant is ExportVariantLite on a lite listing, or empty.
	Variant ExportVariant `json:"variant"`
}

// ExportKind is whether an export covers an hour or a day.
type ExportKind string

// Export kinds.
const (
	ExportKindHourly ExportKind = "hourly"
	ExportKindDaily  ExportKind = "daily"
)

// ExportMeta describes an export without its contents.
type ExportMeta struct {
	ID     string     `json:"id"`
	Stream string     `json:"stream"`
	Kind   ExportKind `json:"kind"`
	// Format is the format of the file that Size, Checksum, and Schema
	// describe: the requested one, else ExportFormatParquet, or
	// ExportFormatTarGz for WHOIS.
	Format ExportFormat `json:"format"`
	// Date is the start of the export's UTC day or hour.
	Date Timestamp `json:"date"`
	// Hour is the UTC hour of an hourly export, or nil for a daily export.
	Hour *int  `json:"hour"`
	Rows int64 `json:"rows"`
	// Size is the size of the file in bytes.
	Size int64 `json:"size"`
	// Checksum is the hex-encoded SHA-256 of the file.
	Checksum  string    `json:"checksum"`
	CreatedAt Timestamp `json:"created_at"`
	// Schema is empty for a WHOIS archive, whose Entities list their own.
	Schema ParquetSchema `json:"schema"`
	// Formats lists every file of the export, its own format first.
	Formats []ExportFile `json:"formats"`
	// Entities lists the parquet files inside a WHOIS archive. It is empty
	// for other exports.
	Entities []ExportEntity `json:"entities"`
	// Variant is ExportVariantLite for a lite export, or empty.
	Variant ExportVariant `json:"variant"`
}

// ExportFormat is the file format of an export. New values may be added.
type ExportFormat string

// Export formats.
const (
	ExportFormatParquet ExportFormat = "parquet"
	// ExportFormatCSV is gzip-compressed, with a header row.
	ExportFormatCSV ExportFormat = "csv"
	// ExportFormatMMDB is a MaxMind DB.
	ExportFormatMMDB  ExportFormat = "mmdb"
	ExportFormatTarGz ExportFormat = "tar.gz"
)

// ExportFile is one file of an export.
type ExportFile struct {
	Format    ExportFormat `json:"format"`
	SizeBytes int64        `json:"size_bytes"`
	// Checksum is the hex-encoded SHA-256 of the file as downloaded.
	Checksum string `json:"checksum"`
}

// ExportEntity is a parquet file inside a WHOIS export archive.
type ExportEntity struct {
	Entity    WhoisEntity `json:"entity"`
	RowCount  int64       `json:"row_count"`
	SizeBytes int64       `json:"size_bytes"`
	// Checksum is the hex-encoded SHA-256 of the parquet file.
	Checksum string `json:"checksum"`
	// Schema is empty in listings and set by ExportMeta.
	Schema ParquetSchema `json:"schema"`
}

// WhoisEntity is a kind of WHOIS record. New values may be added.
type WhoisEntity string

// WHOIS entities.
const (
	WhoisEntityASNs          WhoisEntity = "asns"
	WhoisEntityNetworks      WhoisEntity = "networks"
	WhoisEntityOrganisations WhoisEntity = "organisations" //nolint:misspell // the API's wire value
	WhoisEntityContacts      WhoisEntity = "contacts"
	WhoisEntityMaintainers   WhoisEntity = "maintainers"
	WhoisEntityTransfers     WhoisEntity = "transfers"
	WhoisEntityDelegations   WhoisEntity = "delegations"
)

// ExportVolume is the volume a feed or dataset exported per UTC day.
type ExportVolume struct {
	// Stream is the stream identifier, such as "proxies".
	Stream string `json:"stream"`
	// Days are oldest first. Days without exports are absent.
	Days []ExportVolumeDay `json:"days"`
}

// ExportVolumeDay is the volume exported on one UTC day.
type ExportVolumeDay struct {
	// Date is the UTC day as "YYYY-MM-DD".
	Date string `json:"date"`
	// Kind is ExportKindDaily when the day's daily export is the total, or
	// ExportKindHourly when its hourly exports were summed, as for today.
	Kind ExportKind `json:"kind"`
	// Snapshots is the number of files counted: 1 for a daily export, else
	// the number of hourly exports so far.
	Snapshots int   `json:"snapshots"`
	RowCount  int64 `json:"row_count"`
	SizeBytes int64 `json:"size_bytes"`
}

// ParquetSchema lists the columns of a parquet file.
type ParquetSchema struct {
	// Fields are the columns in file order.
	Fields []ParquetColumn `json:"fields"`
}

// ParquetColumn is a column of a parquet export.
type ParquetColumn struct {
	Name string      `json:"name"`
	Type ParquetType `json:"type"`
}

// ParquetType is the type of a parquet column. New values may be added.
// Dataset exports also use types without constants, such as "double",
// "timestamp", and "list<string>".
type ParquetType string

// Parquet column types.
const (
	ParquetTypeString ParquetType = "string"
	ParquetTypeInt64  ParquetType = "int64"
	ParquetTypeUint32 ParquetType = "uint32"
	ParquetTypeUint64 ParquetType = "uint64"
	ParquetTypeBool   ParquetType = "bool"
	ParquetTypeBytes  ParquetType = "bytes"
)

// ListExports returns one page of the feed's exports, newest first. params
// is optional, and only the first is used. Use AllExports to iterate over
// every page.
func (f *Feed[T]) ListExports(ctx context.Context, params ...ListExportsParams) (*ExportPage, error) {
	var p ListExportsParams
	if len(params) > 0 {
		p = params[0]
	}
	return f.client.listExports(ctx, f.path, p)
}

// AllExports returns an iterator over the feed's exports, newest first. It
// fetches pages lazily and stops fetching when the loop breaks. An error is
// yielded once and ends the iteration. params is optional, and only the
// first is used.
func (f *Feed[T]) AllExports(ctx context.Context, params ...ListExportsParams) iter.Seq2[Export, error] {
	var p ListExportsParams
	if len(params) > 0 {
		p = params[0]
	}
	return f.client.allExports(ctx, f.path, p)
}

// ExportVolume returns the rows and bytes the feed exported per UTC day,
// oldest first. Today is included and still in progress. params is
// optional, and only the first is used.
func (f *Feed[T]) ExportVolume(ctx context.Context, params ...ExportVolumeParams) (*ExportVolume, error) {
	return f.client.exportVolume(ctx, f.path, params)
}

// ExportMeta returns the checksum, size, row count, and parquet schema of an
// export. params is optional, and only the first is used.
func (f *Feed[T]) ExportMeta(ctx context.Context, id string, params ...ExportParams) (*ExportMeta, error) {
	path, query, err := f.export(id, params)
	if err != nil {
		return nil, err
	}
	return doJSON[ExportMeta](ctx, f.client, request{method: http.MethodGet, path: path + "/meta", query: query})
}

// DownloadURL returns a presigned URL for an export's parquet file. The URL
// is valid for 24 hours and grants access without an API key, so treat it as
// a secret. params is optional, and only the first is used.
func (f *Feed[T]) DownloadURL(ctx context.Context, id string, params ...ExportParams) (*url.URL, error) {
	path, query, err := f.export(id, params)
	if err != nil {
		return nil, err
	}
	return f.client.exportURL(ctx, path, query)
}

// Download returns the contents of an export's parquet file. The caller must
// close the returned reader. The API key is never sent to the storage host
// that serves the file, and errors never include the presigned URL. A
// non-success response from either host returns an *APIError. params is
// optional, and only the first is used.
func (f *Feed[T]) Download(ctx context.Context, id string, params ...ExportParams) (io.ReadCloser, error) {
	path, query, err := f.export(id, params)
	if err != nil {
		return nil, err
	}
	return f.client.download(ctx, path, query)
}

func (f *Feed[T]) export(id string, params []ExportParams) (string, url.Values, error) {
	path, err := exportPath(id)
	if err != nil {
		return "", nil, err
	}
	query := url.Values{}
	if len(params) > 0 && params[0].Variant != "" {
		query.Set("variant", string(params[0].Variant))
	}
	return f.path + "/export/" + path, query, nil
}

func (c *Client) listExports(ctx context.Context, path string, p ListExportsParams) (*ExportPage, error) {
	query := url.Values{}
	if p.Limit > 0 {
		query.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		query.Set("cursor", p.Cursor)
	}
	if p.Day != "" {
		query.Set("day", p.Day)
	}
	if p.Variant != "" {
		query.Set("variant", string(p.Variant))
	}
	return doJSON[ExportPage](ctx, c, request{method: http.MethodGet, path: path + "/export", query: query})
}

func (c *Client) allExports(ctx context.Context, path string, start ListExportsParams) iter.Seq2[Export, error] {
	return func(yield func(Export, error) bool) {
		page := start
		seen := map[string]bool{page.Cursor: true}
		for {
			result, err := c.listExports(ctx, path, page)
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

func (c *Client) exportVolume(ctx context.Context, path string, params []ExportVolumeParams) (*ExportVolume, error) {
	query := url.Values{}
	if len(params) > 0 && params[0].Days > 0 {
		query.Set("days", strconv.Itoa(params[0].Days))
	}
	return doJSON[ExportVolume](ctx, c, request{method: http.MethodGet, path: path + "/export/daily", query: query})
}

func (c *Client) exportURL(ctx context.Context, path string, query url.Values) (*url.URL, error) {
	resp, err := c.send(ctx, request{method: http.MethodGet, path: path, query: query})
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, newAPIError(resp)
	}
	if resp.StatusCode < http.StatusMultipleChoices || resp.StatusCode == http.StatusNotModified {
		return nil, fmt.Errorf(
			"synthient: expected a redirect to the export, got status %d",
			resp.StatusCode,
		)
	}
	location, err := resp.Location()
	if err != nil {
		return nil, errors.New("synthient: export redirect has no valid location")
	}
	if location.Scheme != "https" && location.Scheme != c.baseURL.Scheme {
		return nil, errors.New("synthient: export redirect does not use https")
	}
	return location, nil
}

func (c *Client) download(ctx context.Context, path string, query url.Values) (io.ReadCloser, error) {
	location, err := c.exportURL(ctx, path, query)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), http.NoBody)
	if err != nil {
		return nil, errors.New("synthient: invalid export download URL")
	}
	req.Header.Set("User-Agent", c.userAgent)
	// Without an explicit Accept-Encoding, net/http transparently decodes a
	// response sent with Content-Encoding: gzip, which would change the bytes
	// of a .csv.gz file and break its checksum.
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := c.storage.Do(req)
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
