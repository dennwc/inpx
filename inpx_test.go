package inpx

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
)

var testInpxPath = os.Getenv("INPX_PATH")

func TestInpx(t *testing.T) {
	if testInpxPath == "" {
		t.SkipNow()
	}
	index, err := Open(testInpxPath)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, r := range index.Archives {
		total += len(r)
	}
	if total == 0 {
		t.Fatal("no records")
	}
	t.Logf("name: '%s', vers: %v, total: %d", index.Name, index.Version, total)
	for name, recs := range index.Archives {
		t.Logf("record: '%s': %+v", name, recs[0])
		func() {
			file, err := recs[0].File.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if n, err := io.Copy(ioutil.Discard, file); err != nil {
				t.Fatal(err)
			} else {
				t.Log("read", n, "bytes")
			}
		}()
		break
	}
}
