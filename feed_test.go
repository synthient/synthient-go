package synthient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExportPath(t *testing.T) {
	t.Parallel()
	valid := []struct {
		id   string
		want string
	}{
		{id: LatestExport, want: "latest"},
		{id: "2026-05-03", want: "2026-05-03"},
		{id: "2026-05-03/07", want: "2026-05-03/7"},
		{id: "2026-05-03/7", want: "2026-05-03/7"},
		{id: "2026-05-03/23", want: "2026-05-03/23"},
	}
	for _, tt := range valid {
		got, err := exportPath(tt.id)
		if err != nil || got != tt.want {
			t.Errorf("exportPath(%q) = %q, %v, want %q", tt.id, got, err, tt.want)
		}
	}

	invalid := []string{"", "latest/3", "2026-5-3", "2026-02-30", "2026-05-03/24", "2026-05-03/", "../account/me", "2026-05-03/07/meta"}
	for _, id := range invalid {
		_, err := exportPath(id)
		if err == nil {
			t.Errorf("exportPath(%q) succeeded, want an error", id)
		}
	}
}

func TestExportIDHelpers(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 5, 3, 22, 30, 0, 0, time.FixedZone("EST", -5*60*60))
	daily := DailyExport(at)
	if daily != "2026-05-04" {
		t.Errorf("DailyExport = %q, want 2026-05-04", daily)
	}
	hourly := HourlyExport(at)
	if hourly != "2026-05-04/03" {
		t.Errorf("HourlyExport = %q, want 2026-05-04/03", hourly)
	}
}

func TestFeedRoutes(t *testing.T) {
	t.Parallel()
	paths := make(chan string, 1)
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
		writeJSON(w, http.StatusOK, `{}`)
	})
	c := newTestClient(t, srv)

	tests := []struct {
		name   string
		meta   func(context.Context, string, ...ExportParams) (*ExportMeta, error)
		volume func(context.Context, ...ExportVolumeParams) (*ExportVolume, error)
		path   string
	}{
		{name: "proxies", meta: c.Proxies.ExportMeta, volume: c.Proxies.ExportVolume, path: "/api/v4/feeds/proxies"},
		{name: "anonymizers", meta: c.Anonymizers.ExportMeta, volume: c.Anonymizers.ExportVolume, path: "/api/v4/feeds/anonymizers"},
		{name: "torrents", meta: c.Torrents.ExportMeta, volume: c.Torrents.ExportVolume, path: "/api/v4/feeds/torrents"},
		{name: "ja4t", meta: c.JA4T.ExportMeta, volume: c.JA4T.ExportVolume, path: "/api/v4/feeds/ja4t"},
		{name: "helios http", meta: c.HeliosHTTP.ExportMeta, volume: c.HeliosHTTP.ExportVolume, path: "/api/v4/feeds/helios/http"},
		{name: "helios https", meta: c.HeliosHTTPS.ExportMeta, volume: c.HeliosHTTPS.ExportVolume, path: "/api/v4/feeds/helios/https"},
		{name: "helios dns", meta: c.HeliosDNS.ExportMeta, volume: c.HeliosDNS.ExportVolume, path: "/api/v4/feeds/helios/dns"},
		{name: "helios adb", meta: c.HeliosADB.ExportMeta, volume: c.HeliosADB.ExportVolume, path: "/api/v4/feeds/helios/adb"},
	}
	for _, tt := range tests {
		for id, suffix := range map[string]string{LatestExport: "/export/latest/meta", "2026-05-03/07": "/export/2026-05-03/7/meta"} {
			_, err := tt.meta(t.Context(), id)
			if err != nil {
				t.Fatalf("%s ExportMeta(%q): %v", tt.name, id, err)
			}
			got := <-paths
			if got != tt.path+suffix {
				t.Errorf("%s ExportMeta(%q) path = %q, want %q", tt.name, id, got, tt.path+suffix)
			}
		}
		_, err := tt.volume(t.Context())
		if err != nil {
			t.Fatalf("%s ExportVolume: %v", tt.name, err)
		}
		got := <-paths
		if got != tt.path+"/export/daily" {
			t.Errorf("%s ExportVolume path = %q, want %q", tt.name, got, tt.path+"/export/daily")
		}
	}
}

func TestExportMetaRejectsInvalidID(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	})
	c := newTestClient(t, srv)

	_, err := c.Proxies.ExportMeta(t.Context(), "yesterday")
	if err == nil {
		t.Fatal("ExportMeta succeeded with an invalid ID")
	}
	if requests.Load() != 0 {
		t.Errorf("made %d requests, want 0", requests.Load())
	}
}

const exportPageJSON = `{
	"stream": "proxies",
	"feeds": [
		{"kind": "hourly", "date": "2026-05-07", "hour": 21, "size_bytes": 1048576, "row_count": 1200, "checksum": "ab12", "id": "2026-05-07/21", "created_at": 1778187600, "download_path": "/api/v4/feeds/proxies/export/2026-05-07/21", "formats": [{"format": "parquet", "size_bytes": 1048576, "checksum": "ab12"}]},
		{"kind": "daily", "date": "2026-05-06", "size_bytes": 9, "row_count": 3, "checksum": "cd34", "id": "2026-05-06", "created_at": 1778112000, "download_path": "/api/v4/feeds/proxies/export/2026-05-06?variant=lite", "formats": [{"format": "parquet", "size_bytes": 9, "checksum": "cd34"}], "variant": "lite"}
	],
	"next_cursor": "abc"
}`

func TestListExports(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, exportPageJSON)
	})
	c := newTestClient(t, srv)

	page, err := c.Proxies.ListExports(t.Context())
	if err != nil {
		t.Fatalf("ListExports: %v", err)
	}
	if page.Stream != "proxies" || page.NextCursor != "abc" || len(page.Exports) != 2 {
		t.Fatalf("ListExports = %+v", page)
	}
	hourly, daily := page.Exports[0], page.Exports[1]
	if hourly.ID != "2026-05-07/21" || hourly.Hour == nil || *hourly.Hour != 21 || hourly.SizeBytes != 1048576 || hourly.Variant != "" {
		t.Errorf("hourly export = %+v", hourly)
	}
	wantFormats := []ExportFile{{Format: ExportFormatParquet, SizeBytes: 1048576, Checksum: "ab12"}}
	if !reflect.DeepEqual(hourly.Formats, wantFormats) {
		t.Errorf("hourly formats = %+v, want %+v", hourly.Formats, wantFormats)
	}
	if daily.ID != "2026-05-06" || daily.Hour != nil || daily.CreatedAt.Time().Unix() != 1778112000 || daily.Variant != ExportVariantLite {
		t.Errorf("daily export = %+v", daily)
	}
}

func TestListExportsQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		params  []ListExportsParams
		wantURI string
	}{
		{name: "none", wantURI: "/api/v4/feeds/proxies/export"},
		{name: "zero", params: []ListExportsParams{{}}, wantURI: "/api/v4/feeds/proxies/export"},
		{
			name:    "all",
			params:  []ListExportsParams{{Limit: 2, Cursor: "c1", Day: "2026-05-07", Variant: ExportVariantLite}},
			wantURI: "/api/v4/feeds/proxies/export?cursor=c1&day=2026-05-07&limit=2&variant=lite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != tt.wantURI {
					t.Errorf("RequestURI = %q, want %q", r.RequestURI, tt.wantURI)
				}
				writeJSON(w, http.StatusOK, `{"stream":"proxies","feeds":[]}`)
			})
			c := newTestClient(t, srv)

			_, err := c.Proxies.ListExports(t.Context(), tt.params...)
			if err != nil {
				t.Fatalf("ListExports: %v", err)
			}
		})
	}
}

func TestAllExports(t *testing.T) {
	t.Parallel()
	pages := map[string]string{
		"":   `{"stream":"proxies","feeds":[{"id":"2026-05-07/21"},{"id":"2026-05-07/20"}],"next_cursor":"p2"}`,
		"p2": `{"stream":"proxies","feeds":[],"next_cursor":"p3"}`,
		"p3": `{"stream":"proxies","feeds":[{"id":"2026-05-06"}]}`,
	}
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		query := r.URL.Query()
		if query.Get("limit") != "2" || query.Get("day") != "2026-05-07" || query.Get("variant") != "lite" {
			t.Errorf("query = %q, want limit, day, and variant on every page", r.URL.RawQuery)
		}
		writeJSON(w, http.StatusOK, pages[query.Get("cursor")])
	})
	c := newTestClient(t, srv)
	params := ListExportsParams{Limit: 2, Day: "2026-05-07", Variant: ExportVariantLite}

	var ids []string
	for export, err := range c.Proxies.AllExports(t.Context(), params) {
		if err != nil {
			t.Fatalf("AllExports: %v", err)
		}
		ids = append(ids, export.ID)
	}
	want := []string{"2026-05-07/21", "2026-05-07/20", "2026-05-06"}
	if !slices.Equal(ids, want) {
		t.Errorf("AllExports IDs = %v, want %v", ids, want)
	}
	if requests.Load() != 3 {
		t.Errorf("made %d requests, want 3", requests.Load())
	}
	if params.Cursor != "" {
		t.Errorf("AllExports modified params.Cursor to %q", params.Cursor)
	}
}

func TestAllExportsStopsOnBreak(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(w, http.StatusOK, exportPageJSON)
	})
	c := newTestClient(t, srv)

	for _, err := range c.Proxies.AllExports(t.Context()) {
		if err != nil {
			t.Fatalf("AllExports: %v", err)
		}
		break
	}
	if requests.Load() != 1 {
		t.Errorf("made %d requests, want 1", requests.Load())
	}
}

func TestAllExportsErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantItems  int
		wantStatus int
	}{
		{
			name: "repeated cursor",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, http.StatusOK, `{"stream":"proxies","feeds":[{"id":"2026-05-06"}],"next_cursor":"same"}`)
			},
			wantItems: 2,
		},
		{
			name: "second page fails",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("cursor") == "" {
					writeJSON(w, http.StatusOK, `{"stream":"proxies","feeds":[{"id":"2026-05-06"}],"next_cursor":"p2"}`)
					return
				}
				writeJSON(w, http.StatusForbidden, `{"title":"Forbidden","status":403,"detail":"Insufficient scope"}`)
			},
			wantItems:  1,
			wantStatus: http.StatusForbidden,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t, newTestServer(t, tt.handler))

			items := 0
			var iterErr error
			for _, err := range c.Proxies.AllExports(t.Context()) {
				if err != nil {
					iterErr = err
					break
				}
				items++
			}
			if items != tt.wantItems {
				t.Errorf("yielded %d exports, want %d", items, tt.wantItems)
			}
			if iterErr == nil {
				t.Fatal("AllExports yielded no error")
			}
			if tt.wantStatus != 0 {
				apiErr, ok := errors.AsType[*APIError](iterErr)
				if !ok || apiErr.StatusCode != tt.wantStatus {
					t.Errorf("error = %v, want an APIError with status %d", iterErr, tt.wantStatus)
				}
			}
		})
	}
}

func TestExportVariant(t *testing.T) {
	t.Parallel()
	uris := make(chan string, 1)
	storage := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "PAR1")
	})
	api := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		uris <- r.RequestURI
		if strings.HasSuffix(r.URL.Path, "/meta") {
			writeJSON(w, http.StatusOK, `{}`)
			return
		}
		http.Redirect(w, r, storageURL(storage, "/file"), http.StatusTemporaryRedirect)
	})
	c := newTestClient(t, api)

	variants := []struct {
		params ExportParams
		query  string
	}{
		{params: ExportParams{}, query: ""},
		{params: ExportParams{Variant: ExportVariantLite}, query: "?variant=lite"},
	}
	ids := map[string]string{"2026-05-03": "2026-05-03", "2026-05-03/07": "2026-05-03/7"}
	for id, path := range ids {
		path = "/api/v4/feeds/proxies/export/" + path
		for _, v := range variants {
			_, err := c.Proxies.ExportMeta(t.Context(), id, v.params)
			if err != nil {
				t.Fatalf("ExportMeta(%q, %+v): %v", id, v.params, err)
			}
			got := <-uris
			if got != path+"/meta"+v.query {
				t.Errorf("ExportMeta(%q, %+v) RequestURI = %q, want %q", id, v.params, got, path+"/meta"+v.query)
			}

			_, err = c.Proxies.DownloadURL(t.Context(), id, v.params)
			if err != nil {
				t.Fatalf("DownloadURL(%q, %+v): %v", id, v.params, err)
			}
			got = <-uris
			if got != path+v.query {
				t.Errorf("DownloadURL(%q, %+v) RequestURI = %q, want %q", id, v.params, got, path+v.query)
			}

			body, err := c.Proxies.Download(t.Context(), id, v.params)
			if err != nil {
				t.Fatalf("Download(%q, %+v): %v", id, v.params, err)
			}
			_ = body.Close()
			got = <-uris
			if got != path+v.query {
				t.Errorf("Download(%q, %+v) RequestURI = %q, want %q", id, v.params, got, path+v.query)
			}
		}
	}
}

func TestExportVariantNotPublished(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, `{"title":"Not Found","status":404,"detail":"No snapshot found for the requested date"}`)
	})
	c := newTestClient(t, srv)
	lite := ExportParams{Variant: ExportVariantLite}

	_, metaErr := c.Proxies.ExportMeta(t.Context(), LatestExport, lite)
	_, downloadErr := c.Proxies.Download(t.Context(), LatestExport, lite)
	for _, err := range []error{metaErr, downloadErr} {
		apiErr, ok := errors.AsType[*APIError](err)
		if !ok || apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("error = %v, want an APIError with status 404", err)
		}
	}
}

func TestExportVolume(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		params  []ExportVolumeParams
		wantURI string
	}{
		{name: "default", wantURI: "/api/v4/feeds/proxies/export/daily"},
		{name: "zero", params: []ExportVolumeParams{{}}, wantURI: "/api/v4/feeds/proxies/export/daily"},
		{name: "days", params: []ExportVolumeParams{{Days: 7}}, wantURI: "/api/v4/feeds/proxies/export/daily?days=7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.RequestURI != tt.wantURI {
					t.Errorf("RequestURI = %q, want %q", r.RequestURI, tt.wantURI)
				}
				writeJSON(w, http.StatusOK, `{
					"stream": "proxies",
					"days": [
						{"date": "2026-11-16", "kind": "daily", "snapshots": 1, "row_count": 5440452210, "size_bytes": 19629681638},
						{"date": "2026-11-17", "kind": "hourly", "snapshots": 3, "row_count": 64417250, "size_bytes": 880000000}
					]
				}`)
			})
			c := newTestClient(t, srv)

			volume, err := c.Proxies.ExportVolume(t.Context(), tt.params...)
			if err != nil {
				t.Fatalf("ExportVolume: %v", err)
			}
			want := ExportVolume{
				Stream: "proxies",
				Days: []ExportVolumeDay{
					{Date: "2026-11-16", Kind: ExportKindDaily, Snapshots: 1, RowCount: 5440452210, SizeBytes: 19629681638},
					{Date: "2026-11-17", Kind: ExportKindHourly, Snapshots: 3, RowCount: 64417250, SizeBytes: 880000000},
				},
			}
			if !reflect.DeepEqual(*volume, want) {
				t.Errorf("ExportVolume = %+v, want %+v", *volume, want)
			}
		})
	}
}

func TestDecodeExportMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want ExportMeta
	}{
		{
			name: "whois",
			body: `{
				"id": "latest", "stream": "whois", "kind": "daily", "format": "tar.gz", "date": 1794873600,
				"rows": 23481094, "size": 997234948, "checksum": "add0", "created_at": 1794960929,
				"schema": {"fields": []},
				"formats": [{"format": "tar.gz", "size_bytes": 997234948, "checksum": "add0"}],
				"entities": [
					{"entity": "asns", "row_count": 123778, "size_bytes": 5442736, "checksum": "062f", "schema": {"fields": [{"name": "asn", "type": "int64"}, {"name": "updated", "type": "timestamp"}, {"name": "weight", "type": "float"}]}},
					{"entity": "contacts", "row_count": 6459660, "size_bytes": 205447373, "checksum": "cf61", "schema": {"fields": [{"name": "emails", "type": "list<string>"}, {"name": "created", "type": "date"}]}}
				]
			}`,
			want: ExportMeta{
				ID: "latest", Stream: "whois", Kind: ExportKindDaily, Format: ExportFormatTarGz, Date: 1794873600,
				Rows: 23481094, Size: 997234948, Checksum: "add0", CreatedAt: 1794960929,
				Schema:  ParquetSchema{Fields: []ParquetColumn{}},
				Formats: []ExportFile{{Format: ExportFormatTarGz, SizeBytes: 997234948, Checksum: "add0"}},
				Entities: []ExportEntity{
					{
						Entity: WhoisEntityASNs, RowCount: 123778, SizeBytes: 5442736, Checksum: "062f",
						Schema: ParquetSchema{Fields: []ParquetColumn{{Name: "asn", Type: ParquetTypeInt64}, {Name: "updated", Type: "timestamp"}, {Name: "weight", Type: "float"}}},
					},
					{
						Entity: WhoisEntityContacts, RowCount: 6459660, SizeBytes: 205447373, Checksum: "cf61",
						Schema: ParquetSchema{Fields: []ParquetColumn{{Name: "emails", Type: "list<string>"}, {Name: "created", Type: "date"}}},
					},
				},
			},
		},
		{
			name: "null schema fields",
			body: `{"format": "tar.gz", "schema": {"fields": null}}`,
			want: ExportMeta{Format: ExportFormatTarGz},
		},
		{
			name: "geolocation csv",
			body: `{
				"format": "csv",
				"schema": {"fields": [{"name": "latitude", "type": "double"}, {"name": "geoname_id", "type": "int32"}]},
				"formats": [
					{"format": "parquet", "size_bytes": 466431938, "checksum": "9712"},
					{"format": "csv", "size_bytes": 540196440, "checksum": "de74"},
					{"format": "mmdb", "size_bytes": 1874587084, "checksum": "ae71"}
				]
			}`,
			want: ExportMeta{
				Format: ExportFormatCSV,
				Schema: ParquetSchema{Fields: []ParquetColumn{{Name: "latitude", Type: "double"}, {Name: "geoname_id", Type: "int32"}}},
				Formats: []ExportFile{
					{Format: ExportFormatParquet, SizeBytes: 466431938, Checksum: "9712"},
					{Format: ExportFormatCSV, SizeBytes: 540196440, Checksum: "de74"},
					{Format: ExportFormatMMDB, SizeBytes: 1874587084, Checksum: "ae71"},
				},
			},
		},
		{
			name: "mmdb",
			body: `{"format": "mmdb"}`,
			want: ExportMeta{Format: ExportFormatMMDB},
		},
		{
			name: "lite",
			body: `{"format": "parquet", "variant": "lite"}`,
			want: ExportMeta{Format: ExportFormatParquet, Variant: ExportVariantLite},
		},
		{
			name: "unknown values",
			body: `{"format": "zstd", "formats": [{"format": "zstd"}], "entities": [{"entity": "abuse_contacts"}]}`,
			want: ExportMeta{Format: "zstd", Formats: []ExportFile{{Format: "zstd"}}, Entities: []ExportEntity{{Entity: "abuse_contacts"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var meta ExportMeta
			err := json.Unmarshal([]byte(tt.body), &meta)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !reflect.DeepEqual(meta, tt.want) {
				t.Errorf("ExportMeta = %+v, want %+v", meta, tt.want)
			}
		})
	}
}
