package intentproof

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFlushWithoutExporter(t *testing.T) {
	resetRuntimeForTest()
	Flush()
}

func resetRuntimeForTest() {
	clientMu.Lock()
	defer clientMu.Unlock()
	if outbox != nil {
		_ = outbox.Close()
	}
	outbox = nil
	exporter = nil
	instancePrivate = nil
	instanceID = ""
	tenantID = "tnt_default"
}

func TestGetOutboxNotConfigured(t *testing.T) {
	resetRuntimeForTest()
	if _, err := GetOutbox(); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetInstanceIDNotConfigured(t *testing.T) {
	resetRuntimeForTest()
	if _, err := GetInstanceID(); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetPrivateKeyNotConfigured(t *testing.T) {
	resetRuntimeForTest()
	if _, err := GetPrivateKey(); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetPublicKeyNotConfigured(t *testing.T) {
	resetRuntimeForTest()
	if _, err := GetPublicKey(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadOrCreateFreshKeypair(t *testing.T) {
	dir := t.TempDir()
	kp, err := loadOrCreateKeypair(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kp.InstanceID == "" || kp.PrivateKey == "" {
		t.Fatal("expected key material")
	}
}

func TestLoadKeypairRejectsInvalidJSON(t *testing.T) {
	loadKeypairAttempts = 2
	defer func() { loadKeypairAttempts = 50 }()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	if err := os.WriteFile(keyPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadKeypair(keyPath); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadKeypairRejectsDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	if err := os.Mkdir(keyPath, 0o700); err != nil {
		t.Fatal(err)
	}
	loadKeypairAttempts = 2
	defer func() { loadKeypairAttempts = 50 }()
	if _, err := loadKeypair(keyPath); err == nil {
		t.Fatal("expected error reading directory as keypair")
	}
}

func TestWriteKeypairFileFailsOnReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("chmod not supported")
	}
	defer os.Chmod(dir, 0o700)
	err := writeKeypairFile(filepath.Join(dir, "keypair.json"), Keypair{
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		InstanceID: "inst_ro",
	})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestWriteKeypairFileAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	if err := os.WriteFile(keyPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := writeKeypairFile(keyPath, Keypair{
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		InstanceID: "inst_exists",
	})
	if err == nil {
		t.Fatal("expected exist error")
	}
}

func TestWriteKeypairFileWriteFailure(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	if err := os.WriteFile(keyPath, []byte("occupied"), 0o400); err != nil {
		t.Fatal(err)
	}
	err := writeKeypairFile(keyPath, Keypair{
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		InstanceID: "inst_w",
	})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestWriteKeypairFileFailsWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := writeKeypairFile(dir, Keypair{
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		InstanceID: "inst_dir",
	}); err == nil {
		t.Fatal("expected error writing to directory path")
	}
}

func TestDefaultDataDirWhenHomeMissing(t *testing.T) {
	userHomeDirFn = func() (string, error) {
		return "", os.ErrNotExist
	}
	defer func() { userHomeDirFn = os.UserHomeDir }()

	got := DefaultDataDir()
	want := filepath.Join(".intentproof", "sdk-go")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

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

func TestLoadOrCreateKeypairWriteFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("chmod")
	}
	defer os.Chmod(dir, 0o700)
	if _, err := loadOrCreateKeypair(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadOrCreateKeypairStatFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0); err != nil {
		t.Skip("chmod")
	}
	defer os.Chmod(dir, 0o700)
	if _, err := loadOrCreateKeypair(dir); err == nil {
		t.Fatal("expected stat error")
	}
}

func TestWriteKeypairSyncFailure(t *testing.T) {
	keypairFileSyncFn = func(*os.File) error {
		return os.ErrInvalid
	}
	defer func() {
		keypairFileSyncFn = func(f *os.File) error { return f.Sync() }
	}()

	dir := t.TempDir()
	err := writeKeypairFile(filepath.Join(dir, "keypair.json"), Keypair{
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		InstanceID: "inst_sync",
	})
	if err == nil {
		t.Fatal("expected sync error")
	}
}

func TestWriteKeypairWriteFailure(t *testing.T) {
	keypairFileWriteFn = func(*os.File, []byte) (int, error) {
		return 0, os.ErrInvalid
	}
	defer func() {
		keypairFileWriteFn = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
	}()

	dir := t.TempDir()
	err := writeKeypairFile(filepath.Join(dir, "keypair.json"), Keypair{
		PrivateKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		InstanceID: "inst_wr",
	})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestLoadOrCreateKeypairRandFailure(t *testing.T) {
	randReadFn = func([]byte) (int, error) {
		return 0, os.ErrPermission
	}
	defer func() { randReadFn = rand.Read }()

	dir := t.TempDir()
	if _, err := loadOrCreateKeypair(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadKeypairRejectsMissingFields(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "keypair.json")
	payload := `{"privateKey":"","instanceId":""}`
	if err := os.WriteFile(keyPath, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	loadKeypairAttempts = 2
	defer func() { loadKeypairAttempts = 50 }()
	if _, err := loadKeypair(keyPath); err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigureRejectsInvalidPrivateKey(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dataDir, "keypair.json")
	if err := os.WriteFile(key, []byte(`{"privateKey":"not-valid-key","instanceId":"inst_bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Configure(ConfigureOptions{
		DataDir:  dataDir,
		DBPath:   filepath.Join(dir, "outbox.db"),
		TenantID: "tnt_bad_key",
	})
	if err == nil {
		t.Fatal("expected configure error")
	}
}

func TestConfigureFailsEnsureDir(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Configure(ConfigureOptions{
		DataDir: filepath.Join(blocker, "nested"),
		DBPath:  filepath.Join(dir, "outbox.db"),
	})
	if err == nil {
		t.Fatal("expected ensureDir error")
	}
}

func TestConfigureRejectsOutboxDirectoryPath(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	dbDir := filepath.Join(dir, "dbdir")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	err := Configure(ConfigureOptions{
		DataDir: dataDir,
		DBPath:  dbDir,
	})
	if err == nil {
		t.Fatal("expected open outbox error")
	}
}

func TestConfigureFailsWhenDataDirNotCreatable(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Configure(ConfigureOptions{
		DataDir: blocker,
		DBPath:  filepath.Join(dir, "outbox.db"),
	})
	if err == nil {
		t.Fatal("expected configure error")
	}
}

func TestConfigureUsesDefaultOutboxPath(t *testing.T) {
	dir := t.TempDir()
	os.Unsetenv("INTENTPROOF_OUTBOX_PATH")
	if err := Configure(ConfigureOptions{
		DataDir:  dir,
		TenantID: "tnt_default_db",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "outbox.db")); err != nil {
		t.Fatal(err)
	}
}

func TestConfigureUsesTenantFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INTENTPROOF_TENANT_ID", "tnt_env_only")
	if err := Configure(ConfigureOptions{
		DataDir: dir,
		DBPath:  filepath.Join(dir, "outbox.db"),
	}); err != nil {
		t.Fatal(err)
	}
	if GetTenantID() != "tnt_env_only" {
		t.Fatalf("tenant: %s", GetTenantID())
	}
}

func TestConfigureReplacesExporterAndOutbox(t *testing.T) {
	dir := t.TempDir()
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "one.db"),
		DataDir:  dir,
		TenantID: "tnt_reconf",
		IngestURL: "http://127.0.0.1:1/v1/events",
	}); err != nil {
		t.Fatal(err)
	}
	if err := Configure(ConfigureOptions{
		DBPath:   filepath.Join(dir, "two.db"),
		DataDir:  dir,
		TenantID: "tnt_reconf",
	}); err != nil {
		t.Fatal(err)
	}
}
