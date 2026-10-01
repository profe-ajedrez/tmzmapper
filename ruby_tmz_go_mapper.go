package tmzmapper

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	rawURL = "https://raw.githubusercontent.com/rails/rails/main/activesupport/lib/active_support/values/time_zone.rb"
)

// DownloadHash   descarga via petición GET el archivo con el mapeo de tzinfo, y devuelve
// dicho mapeo como map[string]string
func DownloadHash() (map[string]string, error) {
	return downloadHash(http.DefaultClient, rawURL)
}

func downloadHash(client *http.Client, url string) (map[string]string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download time zone mapping: unexpected HTTP status %s", resp.Status)
	}

	return parseTimeZoneMapping(resp.Body)
}

func parseTimeZoneMapping(reader io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(reader)
	mapping := make(map[string]string)
	inMapping := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !inMapping {
			if strings.HasPrefix(line, "MAPPING = {") {
				inMapping = true
			}
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

// SaveMap guarda un map[string]string como un archivo json
func SaveMap(fileName string, mp map[string]string) error {
	body, err := json.MarshalIndent(mp, "  ", "  ")
	if err != nil {
		return err
	}

	f, err := os.Create(fileName)

	if err != nil {
		return err
	}

	defer f.Close()

	_, err2 := f.Write(body)

	if err2 != nil {
		return err
	}
	return nil
}

// TZInfoToIANA devuelve el par IANA según la zona horaria TXInfo pasada como argumento
func TZInfoToIANA(rubyTmz string) (string, error) {
	if !fileExists("./tmzmap.json") {
		mb, err := DownloadHash()
		if err != nil {
			return "", err
		}

		err = SaveMap("./tmzmap.json", mb)
		if err != nil {
			return "", err
		}

		value, ok := mb[rubyTmz]
		if !ok {
			return "", errors.New("couldnt find given tmz")
		}

		return value, nil
	}

	jsonFile, err := os.Open("./tmzmap.json")
	if err != nil {
		return "", err
	}

	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return "", err
	}

	var mb map[string]string
	err = json.Unmarshal(byteValue, &mb)
	if err != nil {
		return "", err
	}

	value, ok := mb[rubyTmz]
	if !ok {
		return "", errors.New("couldnt find given tmz")
	}

	return value, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
