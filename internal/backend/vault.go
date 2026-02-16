// Package backend provides secret storage backend implementations.
//
// This file implements a local encrypted vault backend using SQLite for
// storage and age (filippo.io/age) for per-value encryption. The vault
// is stored at a configurable path (default: ~/.config/envref/vault.db).
//
// Each secret value is encrypted independently using age with a
// passphrase-based recipient derived from the master password. The
// master password is never stored; it must be provided each time the
// vault is accessed.
package backend

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"filippo.io/age"
	"filippo.io/age/armor"
	_ "modernc.org/sqlite"
)

// Default vault database path relative to the user's config directory.
const defaultVaultDir = "envref"
const defaultVaultFile = "vault.db"

// VaultBackend stores secrets in a local SQLite database with per-value
// age encryption. It implements the Backend interface.
//
// The vault uses a single SQLite table to store key-value pairs. Values
// are encrypted using age's scrypt-based passphrase encryption, which
// derives a key from the master password using scrypt with a random salt.
//
// Thread safety is provided via a sync.Mutex.
type VaultBackend struct {
	dbPath     string
	passphrase string
	mu         sync.Mutex
	db         *sql.DB
}

// VaultOption configures a VaultBackend.
type VaultOption func(*VaultBackend)

// WithVaultPath sets the path to the vault database file.
func WithVaultPath(path string) VaultOption {
	return func(v *VaultBackend) {
		v.dbPath = path
	}
}

// NewVaultBackend creates a new VaultBackend with the given passphrase.
// The passphrase is used to encrypt and decrypt secret values via age
// scrypt-based passphrase encryption.
//
// Options can be used to configure the database path. If no path is
// specified, the default (~/.config/envref/vault.db) is used.
//
// The database is created lazily on first access.
func NewVaultBackend(passphrase string, opts ...VaultOption) (*VaultBackend, error) {
	if passphrase == "" {
		return nil, fmt.Errorf("vault passphrase must not be empty")
	}

	v := &VaultBackend{
		passphrase: passphrase,
	}

	for _, opt := range opts {
		opt(v)
	}

	// Use default path if not configured.
	if v.dbPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("vault: determining config directory: %w", err)
		}
		v.dbPath = filepath.Join(configDir, defaultVaultDir, defaultVaultFile)
	}

	return v, nil
}

// Name returns "vault", the identifier used in .envref.yaml configuration
// and ref:// URIs.
func (v *VaultBackend) Name() string {
	return "vault"
}

// Get retrieves and decrypts the secret value for the given key.
// Returns ErrNotFound if the key does not exist.
func (v *VaultBackend) Get(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return "", fmt.Errorf("vault get: %w", err)
	}

	var encrypted string
	err = db.QueryRow("SELECT value FROM secrets WHERE key = ?", key).Scan(&encrypted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("vault get %q: %w", key, err)
	}

	plaintext, err := v.decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("vault get %q: decrypt: %w", key, err)
	}

	return plaintext, nil
}

// Set encrypts and stores a secret value under the given key. If the
// key already exists, its value is overwritten.
func (v *VaultBackend) Set(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault set: %w", err)
	}

	encrypted, err := v.encrypt(value)
	if err != nil {
		return fmt.Errorf("vault set %q: encrypt: %w", key, err)
	}

	_, err = db.Exec(
		"INSERT INTO secrets (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, encrypted,
	)
	if err != nil {
		return fmt.Errorf("vault set %q: %w", key, err)
	}

	return nil
}

// Delete removes the secret for the given key. Returns ErrNotFound if
// the key does not exist.
func (v *VaultBackend) Delete(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault delete: %w", err)
	}

	result, err := db.Exec("DELETE FROM secrets WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("vault delete %q: %w", key, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("vault delete %q: %w", key, err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// List returns all secret keys stored in the vault, sorted alphabetically.
func (v *VaultBackend) List() ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return nil, fmt.Errorf("vault list: %w", err)
	}

	rows, err := db.Query("SELECT key FROM secrets ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("vault list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("vault list: scan: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vault list: %w", err)
	}

	if keys == nil {
		keys = []string{}
	}
	sort.Strings(keys)
	return keys, nil
}

// Close closes the underlying database connection. It is safe to call
// multiple times.
func (v *VaultBackend) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.db != nil {
		err := v.db.Close()
		v.db = nil
		return err
	}
	return nil
}

// open lazily opens (or returns) the SQLite database connection and
// ensures the secrets table exists. Must be called with v.mu held.
func (v *VaultBackend) open() (*sql.DB, error) {
	if v.db != nil {
		return v.db, nil
	}

	// Ensure the parent directory exists.
	dir := filepath.Dir(v.dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating vault directory %q: %w", dir, err)
	}

	db, err := sql.Open("sqlite", v.dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening vault database: %w", err)
	}

	// Create the secrets table if it doesn't exist.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS secrets (
		key   TEXT PRIMARY KEY NOT NULL,
		value TEXT NOT NULL
	)`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing vault schema: %w", err)
	}

	v.db = db
	return v.db, nil
}

// encrypt encrypts a plaintext string using age scrypt passphrase encryption
// and returns the ASCII-armored ciphertext.
func (v *VaultBackend) encrypt(plaintext string) (string, error) {
	recipient, err := age.NewScryptRecipient(v.passphrase)
	if err != nil {
		return "", fmt.Errorf("creating age recipient: %w", err)
	}
	// Use a lower work factor for per-value encryption to keep operations
	// fast. The default scrypt work factor is tuned for file encryption
	// where you do it once; we encrypt many small values.
	recipient.SetWorkFactor(15)

	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)

	writer, err := age.Encrypt(armorWriter, recipient)
	if err != nil {
		return "", fmt.Errorf("creating age writer: %w", err)
	}

	if _, err := io.WriteString(writer, plaintext); err != nil {
		return "", fmt.Errorf("writing plaintext: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing age writer: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return "", fmt.Errorf("closing armor writer: %w", err)
	}

	return buf.String(), nil
}

// decrypt decrypts an ASCII-armored age ciphertext using the vault passphrase
// and returns the plaintext string.
func (v *VaultBackend) decrypt(armored string) (string, error) {
	identity, err := age.NewScryptIdentity(v.passphrase)
	if err != nil {
		return "", fmt.Errorf("creating age identity: %w", err)
	}

	armorReader := armor.NewReader(strings.NewReader(armored))

	reader, err := age.Decrypt(armorReader, identity)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("reading plaintext: %w", err)
	}

	return string(plaintext), nil
}

// DBPath returns the path to the vault database file.
func (v *VaultBackend) DBPath() string {
	return v.dbPath
}
