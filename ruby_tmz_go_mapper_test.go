package tmzmapper

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDownloadHash(t *testing.T) {
	client := responseClient(http.StatusOK, validMappingDocument(), -1)

	mapping, err := downloadHash(client, "https://example.test/time_zone.rb")
	if err != nil {
		t.Fatal(err)
	}
	if got := mapping["Brussels"]; got != "Europe/Brussels" {
		t.Fatalf("Brussels = %q; want Europe/Brussels", got)
	}
	if got := mapping["Nuku'alofa"]; got != "Pacific/Tongatapu" {
		t.Fatalf("Nuku'alofa = %q; want Pacific/Tongatapu", got)
	}
}

func TestDownloadHashRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name    string
		client  *http.Client
		wantErr string
	}{
		{
			name:    "HTTP status",
			client:  responseClient(http.StatusBadGateway, "upstream unavailable", -1),
			wantErr: "unexpected HTTP status 502",
		},
		{
			name:    "declared oversized response",
			client:  responseClient(http.StatusOK, "small", maxMappingDownloadSize+1),
			wantErr: "response is larger",
		},
		{
			name:    "streamed oversized response",
			client:  responseClient(http.StatusOK, strings.Repeat("x", maxMappingDownloadSize+1), -1),
			wantErr: "response is larger",
		},
		{
			name:    "partial mapping",
			client:  responseClient(http.StatusOK, "MAPPING = {\n\"UTC\" => \"Etc/UTC\"\n}", -1),
			wantErr: "want at least 100",
		},
		{
			name: "transport error",
			client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return nil, errors.New("network unavailable")
			})},
			wantErr: "network unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := downloadHash(tt.client, "https://example.test/time_zone.rb")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v; want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func responseClient(statusCode int, body string, contentLength int64) *http.Client {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    statusCode,
			Status:        fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
			ContentLength: contentLength,
			Body:          io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	return client
}

func validMappingDocument() string {
	var document strings.Builder
	document.WriteString(`
class TimeZone
  MAPPING = { # an inline comment
    "Brussels" => "Europe/Brussels",

    # a comment inside the mapping
	    "Eastern Time (US & Canada)" => "America/New_York",
	    "UTC" => "Etc/UTC",
	    "Nuku'alofa" => "Pacific/Tongatapu",
`)
	for index := 0; index < minimumDownloadedTimeZones-4; index++ {
		fmt.Fprintf(&document, "    \"Test Zone %03d\" => \"Etc/Test_%03d\",\n", index, index)
	}
	document.WriteString(`  }

  TWO_DIGITS = Array.new(60)
end
`)
	return document.String()
}

func TestParseTimeZoneMappingErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "missing mapping", input: "class TimeZone\nend", wantErr: "mapping not found"},
		{name: "missing closing brace", input: "MAPPING = {\n\"UTC\" => \"Etc/UTC\"", wantErr: "closing brace not found"},
		{name: "invalid entry", input: "MAPPING = {\ninvalid\n}", wantErr: "invalid time zone mapping entry"},
		{name: "duplicate entry", input: "MAPPING = {\n\"UTC\" => \"Etc/UTC\",\n\"UTC\" => \"Etc/UTC\"\n}", wantErr: "duplicate time zone name"},
		{name: "empty name", input: "MAPPING = {\n\"\" => \"Etc/UTC\"\n}", wantErr: "empty name or identifier"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTimeZoneMapping(strings.NewReader(tt.input))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v; want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestTZInfoToIANARegeneratesInvalidCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "tmzmap.json")
	original := []byte("invalid JSON")
	if err := os.WriteFile(cachePath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	downloads := 0
	download := func() (map[string]string, error) {
		downloads++
		return map[string]string{"Brussels": "Europe/Brussels"}, nil
	}

	got, err := tzInfoToIANA("Brussels", cachePath, download)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Europe/Brussels" {
		t.Fatalf("TZInfoToIANA = %q; want Europe/Brussels", got)
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d; want 1", downloads)
	}
	if _, err := loadMap(cachePath); err != nil {
		t.Fatalf("load regenerated cache: %v", err)
	}
}

func TestTZInfoToIANAKeepsInvalidCacheWhenDownloadFails(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "tmzmap.json")
	original := []byte("invalid JSON")
	if err := os.WriteFile(cachePath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := tzInfoToIANA("Brussels", cachePath, func() (map[string]string, error) {
		return nil, errors.New("download failed")
	})
	if err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Fatalf("error = %v; want download failure", err)
	}

	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("cache changed after failed download: got %q; want %q", got, original)
	}
}

func TestTZInfoToIANAConcurrentInitializationDownloadsOnce(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "tmzmap.json")
	var downloads atomic.Int32
	download := func() (map[string]string, error) {
		downloads.Add(1)
		return map[string]string{"Brussels": "Europe/Brussels"}, nil
	}

	const goroutines = 20
	var waitGroup sync.WaitGroup
	errorsFound := make(chan error, goroutines)
	for index := 0; index < goroutines; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			got, err := tzInfoToIANA("Brussels", cachePath, download)
			if err != nil {
				errorsFound <- err
				return
			}
			if got != "Europe/Brussels" {
				errorsFound <- fmt.Errorf("got %q; want Europe/Brussels", got)
			}
		}()
	}
	waitGroup.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	if got := downloads.Load(); got != 1 {
		t.Fatalf("downloads = %d; want 1", got)
	}
}

func TestTZInfoToIANAUsesValidCacheWithoutDownload(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "tmzmap.json")
	if err := SaveMap(cachePath, map[string]string{"Brussels": "Europe/Brussels"}); err != nil {
		t.Fatal(err)
	}

	got, err := tzInfoToIANA("Brussels", cachePath, func() (map[string]string, error) {
		t.Fatal("download called for a valid cache")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Europe/Brussels" {
		t.Fatalf("TZInfoToIANA = %q; want Europe/Brussels", got)
	}
}

func TestTZInfoToIANADoesNotRegenerateOnCacheReadError(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "tmzmap.json")
	if err := os.Mkdir(cachePath, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := tzInfoToIANA("Brussels", cachePath, func() (map[string]string, error) {
		t.Fatal("download called after a non-recoverable cache error")
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "read time zone cache") {
		t.Fatalf("error = %v; want cache read error", err)
	}
}

func TestSaveMapDoesNotReplaceTargetOnRenameFailure(t *testing.T) {
	targetDirectory := filepath.Join(t.TempDir(), "tmzmap.json")
	if err := os.Mkdir(targetDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(targetDirectory, "marker")
	if err := os.WriteFile(markerPath, []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := SaveMap(targetDirectory, map[string]string{"UTC": "Etc/UTC"})
	if err == nil || !strings.Contains(err.Error(), "replace time zone mapping") {
		t.Fatalf("error = %v; want replacement failure", err)
	}
	if got, err := os.ReadFile(markerPath); err != nil || string(got) != "preserve me" {
		t.Fatalf("target changed after failed replacement: content = %q, error = %v", got, err)
	}
	temporaryFiles, err := filepath.Glob(filepath.Join(filepath.Dir(targetDirectory), ".tmzmap.json.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", temporaryFiles)
	}
}
