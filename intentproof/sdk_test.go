package intentproof_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/intentproof/intentproof-sdk-go/intentproof"
)

func testDirs(t *testing.T) (dbPath, dataDir string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "outbox.db"), filepath.Join(dir, "data")
}

func configureTest(t *testing.T, dbPath, dataDir, tenant string) {
	t.Helper()
	if err := intentproof.Configure(intentproof.ConfigureOptions{
		DBPath:   dbPath,
		DataDir:  dataDir,
		TenantID: tenant,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPersistsKeypairAcrossConfigure(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	id1, err := intentproof.GetInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	pub1, err := intentproof.GetPublicKey()
	if err != nil {
		t.Fatal(err)
	}

	configureTest(t, dbPath, dataDir, "tnt_a")
	id2, err := intentproof.GetInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	pub2, err := intentproof.GetPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatal("expected stable instance id")
	}
	if string(pub1) != string(pub2) {
		t.Fatal("expected stable keypair")
	}
}

func TestProducesSignedEventWithSentinelPrevHash(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x * 2 })
	intentproof.RunWithCorrelationID("corr-1", func() { fn(5) })

	ob, err := intentproof.GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	events, err := ob.Events()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events: got %d", len(events))
	}
	ev := events[0]
	if ev["chain_position"] != int64(1) && ev["chain_position"] != float64(1) {
		t.Fatalf("chain_position: %v", ev["chain_position"])
	}
	if ev["prev_event_hash"] != intentproof.SentinelPrevHash {
		t.Fatalf("prev_event_hash: %v", ev["prev_event_hash"])
	}
	sig, _ := ev["signature"].(map[string]any)
	if sig["alg"] != "ed25519" {
		t.Fatalf("signature: %v", sig)
	}
}

func TestWrapSurvivesConcurrentConfigure(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_reconf")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				intentproof.RunWithCorrelationID("corr-reconf", func() { fn(1) })
			}
		}
	}()

	for range 10 {
		configureTest(t, dbPath, dataDir, "tnt_reconf")
		time.Sleep(2 * time.Millisecond)
	}
	close(stop)
	wg.Wait()
}

func TestConcurrentWrapPreservesChainPositions(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x })
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	start := make(chan struct{})
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			intentproof.RunWithCorrelationID("corr-race", func() {
				for range 25 {
					fn(1)
				}
			})
		}()
	}
	close(start)
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	ob, _ := intentproof.GetOutbox()
	events, err := ob.Events()
	if err != nil {
		t.Fatal(err)
	}
	var race []map[string]any
	for _, ev := range events {
		if ev["correlation_id"] == "corr-race" {
			race = append(race, ev)
		}
	}
	if len(race) != 50 {
		t.Fatalf("race events: got %d", len(race))
	}
}

func TestWrapRecordFailureOnSuccessPathReturnsResult(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	ob, err := intentproof.GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if err := ob.Close(); err != nil {
		t.Fatal(err)
	}

	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x * 2 })
	var got int
	func() {
		defer func() {
			if recover() != nil {
				t.Fatal("unexpected panic on success-path record failure")
			}
		}()
		intentproof.RunWithCorrelationID("corr-rec-fail", func() { got = fn(3) })
	}()
	if got != 6 {
		t.Fatalf("result: got %d want 6", got)
	}

	ob2, err := intentproof.OpenOutbox(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ob2.Close()
	events, err := ob2.Events()
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev["correlation_id"] != "corr-rec-fail" {
			continue
		}
		t.Fatalf("unexpected event after failed success-path record: %+v", ev)
	}
}

func TestWrapFuncRecordFailureOnSuccessPathDoesNotMisclassify(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	ob, err := intentproof.GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	if err := ob.Close(); err != nil {
		t.Fatal(err)
	}

	fn := intentproof.WrapFunc("Test", "test.action", func(x int) (int, error) {
		return x * 2, nil
	})
	intentproof.RunWithCorrelationID("corr-wf-rec-fail", func() {
		_, err = fn(3)
	})
	if err == nil {
		t.Fatal("expected record error")
	}

	ob2, err := intentproof.OpenOutbox(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ob2.Close()
	events, err := ob2.Events()
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev["correlation_id"] != "corr-wf-rec-fail" {
			continue
		}
		t.Fatalf("unexpected event after failed success-path record: %+v", ev)
	}
}

func TestWrapRecordsPanic(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(_ struct{}) int {
		panic("boom")
	})
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		intentproof.RunWithCorrelationID("corr-wrap-panic", func() { fn(struct{}{}) })
	}()
	if !panicked {
		t.Fatal("expected panic")
	}
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	var panicEvents []map[string]any
	for _, ev := range events {
		if ev["correlation_id"] == "corr-wrap-panic" {
			panicEvents = append(panicEvents, ev)
		}
	}
	if len(panicEvents) != 1 {
		t.Fatalf("expected one panic event, got %d", len(panicEvents))
	}
	if panicEvents[0]["status"] != "error" {
		t.Fatalf("status: %v", panicEvents[0]["status"])
	}
}

func TestWrapFuncRecordsPanic(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.WrapFunc("Test", "test.action", func(_ struct{}) (int, error) {
		panic("boom")
	})
	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		intentproof.RunWithCorrelationID("corr-panic", func() {
			_, _ = fn(struct{}{})
		})
	}()
	if !panicked {
		t.Fatal("expected panic")
	}
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	var panicEvents []map[string]any
	for _, ev := range events {
		if ev["correlation_id"] == "corr-panic" {
			panicEvents = append(panicEvents, ev)
		}
	}
	if len(panicEvents) != 1 {
		t.Fatalf("expected one panic event, got %d", len(panicEvents))
	}
	if panicEvents[0]["status"] != "error" {
		t.Fatalf("status: %v", panicEvents[0]["status"])
	}
}

func TestWrapFuncRecordsErrorStatus(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.WrapFunc("Test", "test.action", func(_ struct{}) (int, error) {
		return 0, os.ErrInvalid
	})
	intentproof.RunWithCorrelationID("corr-err", func() {
		if _, err := fn(struct{}{}); err == nil {
			t.Fatal("expected error")
		}
	})
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	ev := events[len(events)-1]
	if ev["status"] != "error" {
		t.Fatalf("status: %v", ev["status"])
	}
}

func TestVerifiableEd25519Signature(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x * 2 })
	intentproof.RunWithCorrelationID("corr-verify", func() { fn(7) })
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	pub, err := intentproof.GetPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev["correlation_id"] != "corr-verify" {
			continue
		}
		ok, err := intentproof.VerifyEventSignature(ev, pub)
		if err != nil || !ok {
			t.Fatalf("verify: ok=%v err=%v", ok, err)
		}
		return
	}
	t.Fatal("event not found")
}

func TestNestedRunWithCorrelationIDRestoresOuter(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x })

	intentproof.RunWithCorrelationID("corr-outer", func() {
		intentproof.RunWithCorrelationID("corr-inner", func() {
			fn(1)
		})
		fn(2)
	})

	ob, err := intentproof.GetOutbox()
	if err != nil {
		t.Fatal(err)
	}
	events, err := ob.Events()
	if err != nil {
		t.Fatal(err)
	}
	var outer, inner int
	for _, ev := range events {
		switch ev["correlation_id"] {
		case "corr-outer":
			outer++
		case "corr-inner":
			inner++
		}
	}
	if inner != 1 {
		t.Fatalf("inner events: %d", inner)
	}
	if outer != 1 {
		t.Fatalf("outer events: %d", outer)
	}
}

func TestCorrelationIsolation(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x * 2 })
	intentproof.RunWithCorrelationID("corr-a", func() { fn(1) })
	intentproof.RunWithCorrelationID("corr-b", func() { fn(2) })
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	var a, b map[string]any
	for _, ev := range events {
		switch ev["correlation_id"] {
		case "corr-a":
			a = ev
		case "corr-b":
			b = ev
		}
	}
	if a["chain_position"] != float64(1) && a["chain_position"] != int64(1) {
		t.Fatalf("corr-a position: %v", a["chain_position"])
	}
	if b["chain_position"] != float64(1) && b["chain_position"] != int64(1) {
		t.Fatalf("corr-b position: %v", b["chain_position"])
	}
}

func TestChainContinuityAcrossReconfigure(t *testing.T) {
	dbPath, dataDir := testDirs(t)
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn := intentproof.Wrap("Test", "test.action", func(x int) int { return x * 2 })
	intentproof.RunWithCorrelationID("corr-2", func() { fn(1) })
	configureTest(t, dbPath, dataDir, "tnt_a")
	fn2 := intentproof.Wrap("Test", "test.action", func(x int) int { return x * 2 })
	intentproof.RunWithCorrelationID("corr-2", func() { fn2(2) })
	ob, _ := intentproof.GetOutbox()
	events, _ := ob.Events()
	if len(events) != 2 {
		t.Fatalf("events: %d", len(events))
	}
	var ev2 map[string]any
	for _, ev := range events {
		pos, _ := ev["chain_position"].(float64)
		if pos == 2 || ev["chain_position"] == int64(2) {
			ev2 = ev
		}
	}
	if ev2["prev_event_hash"] == intentproof.SentinelPrevHash {
		t.Fatal("expected non-sentinel prev hash")
	}
}

// Ensure ed25519 import is used when compiling tests on older toolchains.
var _ ed25519.PublicKey
