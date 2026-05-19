package intentproof

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SDKVersion is reported on emitted execution events.
const SDKVersion = "go@0.1.0"

// ConfigureOptions holds SDK runtime configuration.
type ConfigureOptions struct {
	DBPath    string
	TenantID  string
	DataDir   string
	IngestURL string
}

var (
	clientMu          sync.RWMutex
	instancePrivate   ed25519.PrivateKey
	instanceID        string
	tenantID          = "tnt_default"
	outbox            *Outbox
	exporter          *HTTPExporter
	configuredDataDir string
)

// DefaultDataDir returns the default SDK data directory.
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".intentproof", "sdk-go")
	}
	return filepath.Join(home, ".intentproof", "sdk-go")
}

// Configure initializes keys, outbox, and optional HTTP export.
func Configure(opts ConfigureOptions) error {
	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	if err := ensureDir(dataDir); err != nil {
		return err
	}
	kp, err := loadOrCreateKeypair(dataDir)
	if err != nil {
		return err
	}
	priv, err := LoadPrivateKey(kp.PrivateKey)
	if err != nil {
		return err
	}
	newTenant := opts.TenantID
	if newTenant == "" {
		newTenant = os.Getenv("INTENTPROOF_TENANT_ID")
	}
	if newTenant == "" {
		newTenant = "tnt_default"
	}
	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = os.Getenv("INTENTPROOF_OUTBOX_PATH")
	}
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "outbox.db")
	}
	newOutbox, err := OpenOutbox(dbPath)
	if err != nil {
		return err
	}
	ingest := ResolveIngestURL(opts.IngestURL)
	var newExporter *HTTPExporter
	if ingest != "" {
		newExporter = NewHTTPExporter(ingest)
	}

	clientMu.Lock()
	prevExporter := exporter
	prevOutbox := outbox
	exporter = newExporter
	outbox = newOutbox
	instancePrivate = priv
	instanceID = kp.InstanceID
	tenantID = newTenant
	configuredDataDir = dataDir
	clientMu.Unlock()

	if prevExporter != nil {
		prevExporter.Flush()
	}
	if prevOutbox != nil {
		_ = prevOutbox.Close()
	}
	return nil
}

// Flush waits for in-flight HTTP exports.
func Flush() {
	clientMu.RLock()
	exp := exporter
	clientMu.RUnlock()
	if exp != nil {
		exp.Flush()
	}
}

// GetOutbox returns the configured outbox.
func GetOutbox() (*Outbox, error) {
	clientMu.RLock()
	defer clientMu.RUnlock()
	if outbox == nil {
		return nil, fmt.Errorf("intentproof: SDK not configured: call Configure before use")
	}
	return outbox, nil
}

// GetInstanceID returns the configured instance id.
func GetInstanceID() (string, error) {
	clientMu.RLock()
	defer clientMu.RUnlock()
	if instanceID == "" {
		return "", fmt.Errorf("intentproof: SDK not configured: call Configure before use")
	}
	return instanceID, nil
}

// GetPrivateKey returns the configured signing key.
func GetPrivateKey() (ed25519.PrivateKey, error) {
	clientMu.RLock()
	defer clientMu.RUnlock()
	if instancePrivate == nil {
		return nil, fmt.Errorf("intentproof: SDK not configured: call Configure before use")
	}
	return instancePrivate, nil
}

// GetTenantID returns the configured tenant id.
func GetTenantID() string {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return tenantID
}

// GetPublicKey returns the configured public key.
func GetPublicKey() (ed25519.PublicKey, error) {
	priv, err := GetPrivateKey()
	if err != nil {
		return nil, err
	}
	return priv.Public().(ed25519.PublicKey), nil
}

func getExporter() *HTTPExporter {
	clientMu.RLock()
	defer clientMu.RUnlock()
	return exporter
}
