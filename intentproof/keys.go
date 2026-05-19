package intentproof

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	loadAttempts = 50
	loadRetry    = 20 * time.Millisecond
)

// Keypair holds persisted SDK identity material.
type Keypair struct {
	PrivateKey string `json:"privateKey"`
	InstanceID string `json:"instanceId"`
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

func writeKeypairFile(keyPath string, payload Keypair) error {
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	f, err := os.OpenFile(keyPath, flags, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		_ = os.Remove(keyPath)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = os.Remove(keyPath)
		return err
	}
	return nil
}

func loadKeypair(keyPath string) (Keypair, error) {
	var last error
	for range loadAttempts {
		if err := os.Chmod(keyPath, 0o600); err != nil && !os.IsNotExist(err) {
			// best-effort permission fix
		}
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			last = err
			time.Sleep(loadRetry)
			continue
		}
		if len(raw) == 0 {
			last = errors.New("empty keypair file")
			time.Sleep(loadRetry)
			continue
		}
		var kp Keypair
		if err := json.Unmarshal(raw, &kp); err != nil {
			last = err
			time.Sleep(loadRetry)
			continue
		}
		if kp.PrivateKey == "" || kp.InstanceID == "" {
			last = errors.New("invalid keypair file")
			time.Sleep(loadRetry)
			continue
		}
		return kp, nil
	}
	if last == nil {
		last = errors.New("failed to load keypair")
	}
	return Keypair{}, last
}

func loadOrCreateKeypair(dataDir string) (Keypair, error) {
	keyPath := filepath.Join(dataDir, "keypair.json")
	if _, err := os.Stat(keyPath); err == nil {
		return loadKeypair(keyPath)
	} else if !os.IsNotExist(err) {
		return Keypair{}, err
	}
	seed := make([]byte, 32)
	if _, err := randRead(seed); err != nil {
		return Keypair{}, fmt.Errorf("intentproof: generate key seed: %w", err)
	}
	kp := Keypair{
		PrivateKey: base64.StdEncoding.EncodeToString(seed),
		InstanceID: "inst_" + ulid.Make().String(),
	}
	if err := writeKeypairFile(keyPath, kp); err != nil {
		if errors.Is(err, os.ErrExist) {
			return loadKeypair(keyPath)
		}
		return Keypair{}, err
	}
	return kp, nil
}
