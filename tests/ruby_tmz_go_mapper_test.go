package tests

import (
	"os"
	"testing"

	tmzmapper "github.com/profe-ajedrez/tmzmapper"
)

func TestDownloadHash(t *testing.T) {
	mb, err := tmzmapper.DownloadHash()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	err = tmzmapper.SaveMap("./tmzmap.json", mb)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	value, err := tmzmapper.TZInfoToIANA("Brussels")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	t.Log(value)

	os.Remove("./tmzmap.json")
	value, err = tmzmapper.TZInfoToIANA("Brussels")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	t.Log(value)
}
