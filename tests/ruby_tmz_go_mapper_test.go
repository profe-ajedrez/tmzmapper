package tests

import (
	"os"
	"testing"

	tmzmapper "github.com/profe-ajedrez/tmzmapper"
)

func TestSaveMapAndTZInfoToIANA(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWorkingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	mb := map[string]string{"Brussels": "Europe/Brussels"}
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
}
