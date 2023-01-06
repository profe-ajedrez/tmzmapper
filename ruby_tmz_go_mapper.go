package tmzmapper

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	rawURL = "https://raw.githubusercontent.com/rails/rails/main/activesupport/lib/active_support/values/time_zone.rb"
)

// DownloadHash   descarga via petición GET el archivo con el mapeo de tzinfo, y devuelve
// dicho mapeo como map[string]string
func DownloadHash() (map[string]string, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	str := string(body)

	index := strings.Index(str, "MAPPING")
	str = str[index:]

	index = strings.Index(str, "UTC_OFFSET_WITH_COLON")
	str = strings.ReplaceAll(str[:index], "MAPPING = {", "")
	str = strings.TrimSpace(strings.ReplaceAll(str, "}", ""))

	lines := strings.Split(str, ",")

	gomap := make(map[string]string)
	for _, line := range lines {
		subLine := strings.Split(line, "=>")
		key := strings.ReplaceAll(strings.TrimSpace(subLine[0]), "\"", "")
		value := strings.ReplaceAll(strings.TrimSpace(subLine[1]), "\"", "")
		gomap[key] = value
	}

	return gomap, nil
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
