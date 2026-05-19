package intentproof

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadOrCreateKeypairFileExistsRace(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	var wg sync.WaitGroup
	wg.Add(2)
	var errsMu sync.Mutex
	var errs []error
	for range 2 {
		go func() {
			defer wg.Done()
			_, e := loadOrCreateKeypair(dir)
			if e != nil {
				errsMu.Lock()
				errs = append(errs, e)
				errsMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(errs) > 1 {
		t.Fatalf("errors: %v", errs)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatal(err)
	}
}
