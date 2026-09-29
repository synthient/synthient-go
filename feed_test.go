package synthient

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestExportIDPath(t *testing.T) {
	t.Parallel()
	valid := []struct {
		id   ExportID
		want string
	}{
		{id: LatestExport, want: "latest"},
		{id: "2026-05-03", want: "2026-05-03"},
		{id: "2026-05-03/07", want: "2026-05-03/7"},
		{id: "2026-05-03/7", want: "2026-05-03/7"},
		{id: "2026-05-03/23", want: "2026-05-03/23"},
	}
	for _, tt := range valid {
		got, err := tt.id.path()
		if err != nil || got != tt.want {
			t.Errorf("ExportID(%q).path() = %q, %v, want %q", tt.id, got, err, tt.want)
		}
	}

	invalid := []ExportID{"", "latest/3", "2026-5-3", "2026-02-30", "2026-05-03/24", "2026-05-03/", "../account/me", "2026-05-03/07/meta"}
	for _, id := range invalid {
		_, err := id.path()
		if err == nil {
			t.Errorf("ExportID(%q).path() succeeded, want an error", id)
		}
	}
}

func TestExportIDConstructors(t *testing.T) {
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
		name string
		meta func(context.Context, ExportID) (*ExportMeta, error)
		path string
	}{
		{name: "proxies", meta: c.Proxies.ExportMeta, path: "/api/v4/feeds/proxies"},
		{name: "anonymizers", meta: c.Anonymizers.ExportMeta, path: "/api/v4/feeds/anonymizers"},
		{name: "torrents", meta: c.Torrents.ExportMeta, path: "/api/v4/feeds/torrents"},
		{name: "ja4t", meta: c.JA4T.ExportMeta, path: "/api/v4/feeds/ja4t"},
		{name: "helios http", meta: c.HeliosHTTP.ExportMeta, path: "/api/v4/feeds/helio/http"},
		{name: "helios https", meta: c.HeliosHTTPS.ExportMeta, path: "/api/v4/feeds/helio/https"},
		{name: "helios dns", meta: c.HeliosDNS.ExportMeta, path: "/api/v4/feeds/helio/dns"},
		{name: "helios adb", meta: c.HeliosADB.ExportMeta, path: "/api/v4/feeds/helio/adb"},
	}
	for _, tt := range tests {
		for id, suffix := range map[ExportID]string{LatestExport: "/export/latest/meta", "2026-05-03/07": "/export/2026-05-03/7/meta"} {
			_, err := tt.meta(t.Context(), id)
			if err != nil {
				t.Fatalf("%s ExportMeta(%q): %v", tt.name, id, err)
			}
			got := <-paths
			if got != tt.path+suffix {
				t.Errorf("%s ExportMeta(%q) path = %q, want %q", tt.name, id, got, tt.path+suffix)
			}
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
		{"kind": "hourly", "date": "2026-05-07", "hour": 21, "size_bytes": 1048576, "row_count": 1200, "checksum": "ab12", "id": "2026-05-07/21", "created_at": 1778187600, "download_path": "/api/v4/feeds/proxies/export/2026-05-07/21"},
		{"kind": "daily", "date": "2026-05-06", "size_bytes": 9, "row_count": 3, "checksum": "cd34", "id": "2026-05-06", "created_at": 1778112000, "download_path": "/api/v4/feeds/proxies/export/2026-05-06"}
	],
	"next_cursor": "abc"
}`

func TestListExports(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		want := "/api/v4/feeds/proxies/export?cursor=c1&limit=2"
		if r.RequestURI != want {
			t.Errorf("RequestURI = %q, want %q", r.RequestURI, want)
		}
		writeJSON(w, http.StatusOK, exportPageJSON)
	})
	c := newTestClient(t, srv)

	page, err := c.Proxies.ListExports(t.Context(), &ListExportsParams{Limit: 2, Cursor: "c1"})
	if err != nil {
		t.Fatalf("ListExports: %v", err)
	}
	if page.Stream != "proxies" || page.NextCursor != "abc" || len(page.Exports) != 2 {
		t.Fatalf("ListExports = %+v", page)
	}
	hourly, daily := page.Exports[0], page.Exports[1]
	if hourly.ID != "2026-05-07/21" || hourly.Hour == nil || *hourly.Hour != 21 || hourly.SizeBytes != 1048576 {
		t.Errorf("hourly export = %+v", hourly)
	}
	if daily.ID != "2026-05-06" || daily.Hour != nil || daily.CreatedAt.Time().Unix() != 1778112000 {
		t.Errorf("daily export = %+v", daily)
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
		if r.URL.Query().Get("limit") != "2" {
			t.Errorf("limit = %q, want 2", r.URL.Query().Get("limit"))
		}
		writeJSON(w, http.StatusOK, pages[r.URL.Query().Get("cursor")])
	})
	c := newTestClient(t, srv)
	params := &ListExportsParams{Limit: 2}

	var ids []ExportID
	for export, err := range c.Proxies.AllExports(t.Context(), params) {
		if err != nil {
			t.Fatalf("AllExports: %v", err)
		}
		ids = append(ids, export.ID)
	}
	want := []ExportID{"2026-05-07/21", "2026-05-07/20", "2026-05-06"}
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

	for range c.Proxies.AllExports(t.Context(), nil) {
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
			for _, err := range c.Proxies.AllExports(t.Context(), nil) {
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
