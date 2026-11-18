package synthient

import (
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"time"
)

// Dataset is a Synthient dataset: one export per UTC day, with no stream and
// no hourly exports. Access datasets through Client.Geolocation and
// Client.WHOIS.
//
// An export ID is LatestExport, the newest day, or a UTC day as
// "YYYY-MM-DD". Export.ID values can be passed as they are. Requests for a
// day that is not published yet return a 404 *APIError.
type Dataset struct {
	client *Client
	path   string
}

// ListDatasetExportsParams holds optional parameters for listing a dataset's
// exports.
type ListDatasetExportsParams struct {
	// Limit is the page size, from 1 to 500. Zero uses the API default of 100.
	Limit int
	// Cursor continues a listing from a previous page's NextCursor.
	Cursor string
	// Day lists only the export of a UTC day, as "YYYY-MM-DD".
	Day string
}

// DatasetExportParams holds optional parameters for a dataset export.
type DatasetExportParams struct {
	// Format selects the file of the day. Geolocation has
	// ExportFormatParquet, its default, ExportFormatCSV, and ExportFormatMMDB.
	// WHOIS has only ExportFormatTarGz. A format the day lacks returns a 404
	// *APIError.
	Format ExportFormat
}

// ListExports returns one page of the dataset's exports, newest first.
// params is optional, and only the first is used. Use AllExports to iterate
// over every page.
func (d *Dataset) ListExports(ctx context.Context, params ...ListDatasetExportsParams) (*ExportPage, error) {
	var p ListDatasetExportsParams
	if len(params) > 0 {
		p = params[0]
	}
	return d.client.listExports(ctx, d.path, ListExportsParams{Limit: p.Limit, Cursor: p.Cursor, Day: p.Day})
}

// AllExports returns an iterator over the dataset's exports, newest first.
// It fetches pages lazily and stops fetching when the loop breaks. An error
// is yielded once and ends the iteration. params is optional, and only the
// first is used.
func (d *Dataset) AllExports(ctx context.Context, params ...ListDatasetExportsParams) iter.Seq2[Export, error] {
	var p ListDatasetExportsParams
	if len(params) > 0 {
		p = params[0]
	}
	return d.client.allExports(ctx, d.path, ListExportsParams{Limit: p.Limit, Cursor: p.Cursor, Day: p.Day})
}

// ExportVolume returns the rows and bytes the dataset exported per UTC day,
// oldest first. Today is included and still in progress. params is
// optional, and only the first is used.
func (d *Dataset) ExportVolume(ctx context.Context, params ...ExportVolumeParams) (*ExportVolume, error) {
	return d.client.exportVolume(ctx, d.path, params)
}

// ExportMeta returns the checksum, size, and row count of an export's file,
// and lists every file of the day. params is optional, and only the first is
// used.
func (d *Dataset) ExportMeta(ctx context.Context, id string, params ...DatasetExportParams) (*ExportMeta, error) {
	path, query, err := d.export(id, params)
	if err != nil {
		return nil, err
	}
	return doJSON[ExportMeta](ctx, d.client, request{method: http.MethodGet, path: path + "/meta", query: query})
}

// DownloadURL returns a presigned URL for an export's file. The URL is valid
// for 24 hours and grants access without an API key, so treat it as a
// secret. params is optional, and only the first is used.
func (d *Dataset) DownloadURL(ctx context.Context, id string, params ...DatasetExportParams) (*url.URL, error) {
	path, query, err := d.export(id, params)
	if err != nil {
		return nil, err
	}
	return d.client.exportURL(ctx, path, query)
}

// Download returns the contents of an export's file exactly as served: a csv
// file is gzip-compressed, and a WHOIS file is a tar.gz archive. The caller
// must close the returned reader. The API key is never sent to the storage
// host that serves the file, and errors never include the presigned URL. A
// non-success response from either host returns an *APIError. params is
// optional, and only the first is used.
func (d *Dataset) Download(ctx context.Context, id string, params ...DatasetExportParams) (io.ReadCloser, error) {
	path, query, err := d.export(id, params)
	if err != nil {
		return nil, err
	}
	return d.client.download(ctx, path, query)
}

func (d *Dataset) export(id string, params []DatasetExportParams) (string, url.Values, error) {
	_, err := time.Parse(time.DateOnly, id)
	if id != LatestExport && err != nil {
		return "", nil, fmt.Errorf("synthient: invalid export ID %q", id)
	}
	query := url.Values{}
	if len(params) > 0 && params[0].Format != "" {
		query.Set("format", string(params[0].Format))
	}
	return d.path + "/export/" + id, query, nil
}
