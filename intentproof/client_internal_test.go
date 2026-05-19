package intentproof

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentConfigureDoesNotLeakOutbox(t *testing.T) {
	dir := t.TempDir()
	dbA := filepath.Join(dir, "a.db")
	dbB := filepath.Join(dir, "b.db")
	dataDir := filepath.Join(dir, "data")

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, dbPath := range []string{dbA, dbB} {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			errs <- Configure(ConfigureOptions{
				DBPath:   path,
				DataDir:  dataDir,
				TenantID: "tnt_parallel_cfg",
			})
		}(dbPath)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	ob, err := GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if ob == nil {
		t.Fatal("expected configured outbox")
	}
}
