package versionpolicy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransportHandlesVersionUpgradeNotice(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header string
		want   []string
	}{
		{name: "outdated success", status: http.StatusOK, header: "0.5.0", want: []string{"0.5.0"}},
		{name: "current success", status: http.StatusOK},
		{name: "error response", status: http.StatusUpgradeRequired, header: "0.5.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newLatestVersionServer(t, tt.status, tt.header)

			var handled []string
			client := &http.Client{Transport: Transport{
				HandleVersionUpgradeNotice: func(latest string) { handled = append(handled, latest) },
			}}
			resp, err := client.Get(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if len(handled) != len(tt.want) || (len(tt.want) == 1 && handled[0] != tt.want[0]) {
				t.Fatalf("handled %q, want %q", handled, tt.want)
			}
		})
	}
}

func newLatestVersionServer(t *testing.T, status int, latest string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if latest != "" {
			w.Header().Set("X-Tollbit-Cli-Latest-Version", latest)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}
