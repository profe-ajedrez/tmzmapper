package tmzmapper

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	rawURL                     = "https://raw.githubusercontent.com/rails/rails/v8.1.4/activesupport/lib/active_support/values/time_zone.rb"
	defaultHTTPTimeout         = 10 * time.Second
	maxMappingDownloadSize     = 1 << 20
	minimumDownloadedTimeZones = 100
)

var (
	defaultHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	initializationMu  sync.Mutex
	errInvalidCache   = errors.New("invalid time zone cache")
)

// DownloadHash   descarga via petición GET el archivo con el mapeo de tzinfo, y devuelve
// dicho mapeo como map[string]string
func DownloadHash() (map[string]string, error) {
	return downloadHash(defaultHTTPClient, rawURL)
}

func downloadHash(client *http.Client, url string) (map[string]string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download time zone mapping: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download time zone mapping: unexpected HTTP status %s", resp.Status)
	}
	if resp.ContentLength > maxMappingDownloadSize {
		return nil, fmt.Errorf("download time zone mapping: response is larger than %d bytes", maxMappingDownloadSize)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMappingDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("download time zone mapping: read response: %w", err)
	}
	if len(body) > maxMappingDownloadSize {
		return nil, fmt.Errorf("download time zone mapping: response is larger than %d bytes", maxMappingDownloadSize)
	}

	mapping, err := parseTimeZoneMapping(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("download time zone mapping: %w", err)
	}
	if err := validateDownloadedMapping(mapping); err != nil {
		return nil, fmt.Errorf("download time zone mapping: %w", err)
	}
	return mapping, nil
}

func parseTimeZoneMapping(reader io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(reader)
	mapping := make(map[string]string)
	inMapping := false

	for scanner.Scan() {
		line := strings.TrimSpace(stripRubyComment(scanner.Text()))
		if !inMapping {
			if line == "MAPPING = {" {
				inMapping = true
			}
			continue
		}

		if line == "" {
			continue
		}
		if line == "}" {
			if len(mapping) == 0 {
				return nil, errors.New("time zone mapping is empty")
			}
			return mapping, nil
		}

		line = strings.TrimSuffix(line, ",")
		parts := strings.SplitN(line, "=>", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid time zone mapping entry: %q", line)
		}

		key, err := strconv.Unquote(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid time zone name in %q: %w", line, err)
		}
		value, err := strconv.Unquote(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid time zone identifier in %q: %w", line, err)
		}
		if key == "" || value == "" {
			return nil, fmt.Errorf("time zone mapping contains an empty name or identifier in %q", line)
		}
		if _, exists := mapping[key]; exists {
			return nil, fmt.Errorf("duplicate time zone name %q", key)
		}
		mapping[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read time zone mapping: %w", err)
	}
	if !inMapping {
		return nil, errors.New("time zone mapping not found")
	}
	return nil, errors.New("time zone mapping closing brace not found")
}

func stripRubyComment(line string) string {
	inString := false
	escaped := false
	for i, character := range line {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && inString {
			escaped = true
			continue
		}
		if character == '"' {
			inString = !inString
			continue
		}
		if character == '#' && !inString {
			return line[:i]
		}
	}
	return line
}

func validateDownloadedMapping(mapping map[string]string) error {
	if len(mapping) < minimumDownloadedTimeZones {
		return fmt.Errorf("mapping contains %d entries; want at least %d", len(mapping), minimumDownloadedTimeZones)
	}

	required := map[string]string{
		"UTC":                        "Etc/UTC",
		"Brussels":                   "Europe/Brussels",
		"Eastern Time (US & Canada)": "America/New_York",
	}
	for name, identifier := range required {
		if mapping[name] != identifier {
			return fmt.Errorf("mapping does not contain required entry %q => %q", name, identifier)
		}
	}
	return nil
}

// SaveMap guarda un map[string]string como un archivo json
func SaveMap(fileName string, mp map[string]string) error {
	body, err := json.MarshalIndent(mp, "  ", "  ")
	if err != nil {
		return fmt.Errorf("encode time zone mapping: %w", err)
	}

	directory := filepath.Dir(fileName)
	temporaryFile, err := os.CreateTemp(directory, "."+filepath.Base(fileName)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary time zone mapping: %w", err)
	}
	temporaryName := temporaryFile.Name()
	keepTemporaryFile := true
	defer func() {
		if keepTemporaryFile {
			temporaryFile.Close()
			os.Remove(temporaryName)
		}
	}()

	if err := temporaryFile.Chmod(0o644); err != nil {
		return fmt.Errorf("set time zone mapping permissions: %w", err)
	}
	bytesWritten, err := temporaryFile.Write(body)
	if err != nil {
		return fmt.Errorf("write time zone mapping: %w", err)
	}
	if bytesWritten != len(body) {
		return fmt.Errorf("write time zone mapping: %w", io.ErrShortWrite)
	}
	if err := temporaryFile.Sync(); err != nil {
		return fmt.Errorf("sync time zone mapping: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close time zone mapping: %w", err)
	}
	if err := os.Rename(temporaryName, fileName); err != nil {
		return fmt.Errorf("replace time zone mapping: %w", err)
	}
	keepTemporaryFile = false
	return nil
}

// TZInfoToIANA devuelve el par IANA según la zona horaria TXInfo pasada como argumento
func TZInfoToIANA(rubyTmz string) (string, error) {
	return tzInfoToIANA(rubyTmz, "./tmzmap.json", DownloadHash)
}

func tzInfoToIANA(rubyTmz, cachePath string, download func() (map[string]string, error)) (string, error) {
	mapping, err := loadMap(cachePath)
	if err == nil {
		return lookupTimeZone(mapping, rubyTmz)
	}
	if !isRecoverableCacheError(err) {
		return "", err
	}

	initializationMu.Lock()
	defer initializationMu.Unlock()

	// Another goroutine may have regenerated the cache while this one waited.
	mapping, err = loadMap(cachePath)
	if err == nil {
		return lookupTimeZone(mapping, rubyTmz)
	}
	if !isRecoverableCacheError(err) {
		return "", err
	}

	mapping, err = download()
	if err != nil {
		return "", err
	}
	if err := SaveMap(cachePath, mapping); err != nil {
		return "", err
	}
	return lookupTimeZone(mapping, rubyTmz)
}

func loadMap(fileName string) (map[string]string, error) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("open time zone cache: %w", err)
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(io.LimitReader(jsonFile, maxMappingDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("read time zone cache: %w", err)
	}
	if len(byteValue) > maxMappingDownloadSize {
		return nil, fmt.Errorf("%w: file is larger than %d bytes", errInvalidCache, maxMappingDownloadSize)
	}

	var mapping map[string]string
	if err := json.Unmarshal(byteValue, &mapping); err != nil {
		return nil, fmt.Errorf("%w: decode JSON: %v", errInvalidCache, err)
	}
	if len(mapping) == 0 {
		return nil, fmt.Errorf("%w: mapping is empty", errInvalidCache)
	}
	for name, identifier := range mapping {
		if name == "" || identifier == "" {
			return nil, fmt.Errorf("%w: mapping contains an empty name or identifier", errInvalidCache)
		}
	}
	return mapping, nil
}

func lookupTimeZone(mapping map[string]string, rubyTmz string) (string, error) {
	value, ok := mapping[rubyTmz]
	if !ok {
		return "", fmt.Errorf("time zone %q not found", rubyTmz)
	}
	return value, nil
}

func isRecoverableCacheError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, errInvalidCache)
}
