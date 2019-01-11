package faggot

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"
)

func TestDataProvider(t *testing.T) {
	workingDir := path.Join(os.TempDir(), fmt.Sprintf("faggot_bot_data_%s", randStringRunes(4)))
	defer func() {
		os.RemoveAll(workingDir)
	}()
	t.Logf("Using data directory: %s", workingDir)

	dp := NewDataProvider(workingDir)
	out := make(chan int, 10)
	count := 100

	for i := 0; i < count; i++ {
		go func(out chan int, i int) {
			filename := fmt.Sprintf("filename_%03d.json", i)
			dp.loadJSON(filename)
			data := []byte(`{"id":1488, "first_name": "Adolf", "last_name": "Hitler"}`)
			dp.saveJSON(filename, data)
			out <- i
		}(out, i)
	}

	go func() {
		time.Sleep(1 * time.Second)
		close(out)
	}()

	var arr []int
	for i := range out {
		arr = append(arr, i)
	}

	if len(arr) < count {
		t.Error("Not all files created")
	}
}