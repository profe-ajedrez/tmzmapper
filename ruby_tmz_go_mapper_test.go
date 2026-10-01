package tmzmapper

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestDownloadHash(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body: io.NopCloser(strings.NewReader(`
class TimeZone
  MAPPING = { # rubocop:disable Style/MutableConstant
    "Brussels" => "Europe/Brussels",
    "Nuku'alofa" => "Pacific/Tongatapu"
  }

  TWO_DIGITS = Array.new(60)
end
`)),
		}, nil
	})}

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

func TestParseTimeZoneMappingErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "missing mapping", input: "class TimeZone\nend", wantErr: "mapping not found"},
		{name: "missing closing brace", input: "MAPPING = {\n\"UTC\" => \"Etc/UTC\"", wantErr: "closing brace not found"},
		{name: "invalid entry", input: "MAPPING = {\ninvalid\n}", wantErr: "invalid time zone mapping entry"},
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
