package progress

import (
	"bytes"
	"testing"
	"time"
)

func TestProgressBar(t *testing.T) {
	data := make([]byte, 1024)

	dst := &bytes.Buffer{}

	progress := New()
	bar := progress.NewBar(int64(len(data)), "copying Data")

	src := bytes.NewReader(data)

	_, err := progress.Execute(dst, src, int64(len(data)), bar)
	if err != nil {
		t.Fatalf("error during copy: %v", err)
	}

	progress.Wait()

	if !bytes.Equal(data, dst.Bytes()) {
		t.Fatalf("data mismatch: expected %v, got %v", data, dst.Bytes())
	}
	if !bar.Completed() {
		t.Fatalf("progress bar not completed: %v/%v", bar.Current(), int64(len(data)))
	}

	time.Sleep(1 * time.Second)
}
