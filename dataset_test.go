package synthient

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDatasetRoutes(t *testing.T) {
	t.Parallel()
	uris := make(chan string, 1)
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		uris <- r.RequestURI
		if strings.HasSuffix(r.URL.Path, "/export/2026-11-17") || strings.HasSuffix(r.URL.Path, "/export/latest") {
			http.Redirect(w, r, "https://storage.example/file?X-Amz-Signature=secret", http.StatusTemporaryRedirect)
			return
		}
		writeJSON(w, http.StatusOK, `{}`)
	})
	c := newTestClient(t, srv)

	calls := []struct {
		name string
		call func(*Dataset) error
		want string
	}{
		{
			name: "ListExports",
			call: func(d *Dataset) error {
				_, err := d.ListExports(t.Context())
				return err
			},
			want: "/export",
		},
		{
			name: "ListExports with params",
			call: func(d *Dataset) error {
				_, err := d.ListExports(t.Context(), ListDatasetExportsParams{Limit: 5, Cursor: "c1", Day: "2026-11-17"})
				return err
			},
			want: "/export?cursor=c1&day=2026-11-17&limit=5",
		},
		{
			name: "ExportVolume",
			call: func(d *Dataset) error {
				_, err := d.ExportVolume(t.Context(), ExportVolumeParams{Days: 30})
				return err
			},
			want: "/export/daily?days=30",
		},
		{
			name: "ExportMeta",
			call: func(d *Dataset) error {
				_, err := d.ExportMeta(t.Context(), LatestExport)
				return err
			},
			want: "/export/latest/meta",
		},
		{
			name: "ExportMeta with format",
			call: func(d *Dataset) error {
				_, err := d.ExportMeta(t.Context(), "2026-11-17", DatasetExportParams{Format: ExportFormatMMDB})
				return err
			},
			want: "/export/2026-11-17/meta?format=mmdb",
		},
		{
			name: "DownloadURL",
			call: func(d *Dataset) error {
				_, err := d.DownloadURL(t.Context(), "2026-11-17", DatasetExportParams{})
				return err
			},
			want: "/export/2026-11-17",
		},
		{
			name: "DownloadURL with format",
			call: func(d *Dataset) error {
				_, err := d.DownloadURL(t.Context(), LatestExport, DatasetExportParams{Format: ExportFormatTarGz})
				return err
			},
			want: "/export/latest?format=tar.gz",
		},
	}
	datasets := map[string]*Dataset{"/api/v4/feeds/geolocation": c.Geolocation, "/api/v4/feeds/whois": c.WHOIS}
	for path, dataset := range datasets {
		for _, tt := range calls {
			err := tt.call(dataset)
			if err != nil {
				t.Fatalf("%s %s: %v", path, tt.name, err)
			}
			got := <-uris
			if got != path+tt.want {
				t.Errorf("%s RequestURI = %q, want %q", tt.name, got, path+tt.want)
			}
		}
	}
}

func TestDatasetRejectsInvalidID(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := newTestServer(t, func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	})
	c := newTestClient(t, srv)

	for _, id := range []string{"2026-11-17/3", "2026-11-17/03", "yesterday", "", "daily", "../account/me"} {
		_, err := c.Geolocation.ExportMeta(t.Context(), id)
		if err == nil {
			t.Errorf("ExportMeta(%q) succeeded, want an error", id)
		}
		_, err = c.Geolocation.DownloadURL(t.Context(), id)
		if err == nil {
			t.Errorf("DownloadURL(%q) succeeded, want an error", id)
		}
		body, err := c.WHOIS.Download(t.Context(), id)
		if body != nil {
			_ = body.Close()
		}
		if err == nil {
			t.Errorf("Download(%q) succeeded, want an error", id)
		}
	}
	if requests.Load() != 0 {
		t.Errorf("made %d requests, want 0", requests.Load())
	}
}

func TestDatasetAllExports(t *testing.T) {
	t.Parallel()
	pages := map[string]string{
		"": `{"stream":"geolocation","feeds":[{
			"id": "2026-11-17", "kind": "daily", "date": "2026-11-17", "size_bytes": 466431938, "row_count": 27456467,
			"checksum": "9712", "created_at": 1794960045, "download_path": "/api/v4/feeds/geolocation/export/2026-11-17",
			"formats": [
				{"format": "parquet", "size_bytes": 466431938, "checksum": "9712"},
				{"format": "csv", "size_bytes": 540196440, "checksum": "de74"},
				{"format": "mmdb", "size_bytes": 1874587084, "checksum": "ae71"}
			]
		}],"next_cursor":"p2"}`,
		"p2": `{"stream":"geolocation","feeds":[]}`,
	}
	var requests atomic.Int32
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		query := r.URL.Query()
		if r.URL.Path != "/api/v4/feeds/geolocation/export" || query.Get("day") != "2026-11-17" || query.Has("variant") {
			t.Errorf("request = %s, want the geolocation listing with day and no variant", r.RequestURI)
		}
		writeJSON(w, http.StatusOK, pages[query.Get("cursor")])
	})
	c := newTestClient(t, srv)

	var exports []Export
	for export, err := range c.Geolocation.AllExports(t.Context(), ListDatasetExportsParams{Day: "2026-11-17"}) {
		if err != nil {
			t.Fatalf("AllExports: %v", err)
		}
		exports = append(exports, export)
	}
	if requests.Load() != 2 || len(exports) != 1 {
		t.Fatalf("got %d exports over %d requests, want 1 over 2", len(exports), requests.Load())
	}
	want := []ExportFile{
		{Format: ExportFormatParquet, SizeBytes: 466431938, Checksum: "9712"},
		{Format: ExportFormatCSV, SizeBytes: 540196440, Checksum: "de74"},
		{Format: ExportFormatMMDB, SizeBytes: 1874587084, Checksum: "ae71"},
	}
	if exports[0].ID != "2026-11-17" || !reflect.DeepEqual(exports[0].Formats, want) {
		t.Errorf("export = %+v", exports[0])
	}
}

func TestDatasetDownloadReturnsFileAsServed(t *testing.T) {
	t.Parallel()
	var file bytes.Buffer
	zw := gzip.NewWriter(&file)
	_, err := io.WriteString(zw, "network,city\n1.0.0.0/24,Sydney\n")
	if err != nil {
		t.Fatal(err)
	}
	err = zw.Close()
	if err != nil {
		t.Fatal(err)
	}

	storage := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "" {
			t.Error("storage host received the API key")
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(file.Bytes())
	})
	var apiURI atomic.Value
	api := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		apiURI.Store(r.RequestURI)
		http.Redirect(w, r, storageURL(storage, "/geolocation/2026-11-17.csv.gz"), http.StatusTemporaryRedirect)
	})
	c := newTestClient(t, api)

	body, err := c.Geolocation.Download(t.Context(), "2026-11-17", DatasetExportParams{Format: ExportFormatCSV})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if !bytes.Equal(data, file.Bytes()) {
		t.Errorf("Download returned %d bytes, want the %d gzip bytes as served", len(data), file.Len())
	}
	if apiURI.Load() != "/api/v4/feeds/geolocation/export/2026-11-17?format=csv" {
		t.Errorf("API RequestURI = %v", apiURI.Load())
	}
}
