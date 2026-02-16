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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	_ "modernc.org/sqlite"

	"github.com/xcke/envref/internal/secret"
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
	passphrase []byte
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
		passphrase: []byte(passphrase),
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
// Returns ErrNotFound if the key does not exist, or ErrVaultLocked if
// the vault is locked.
func (v *VaultBackend) Get(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return "", fmt.Errorf("vault get: %w", err)
	}

	if err := v.checkLocked(db); err != nil {
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
// key already exists, its value is overwritten. Returns ErrVaultLocked
// if the vault is locked.
func (v *VaultBackend) Set(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault set: %w", err)
	}

	if err := v.checkLocked(db); err != nil {
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
// the key does not exist, or ErrVaultLocked if the vault is locked.
func (v *VaultBackend) Delete(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault delete: %w", err)
	}

	if err := v.checkLocked(db); err != nil {
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
// Returns ErrVaultLocked if the vault is locked.
func (v *VaultBackend) List() ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return nil, fmt.Errorf("vault list: %w", err)
	}

	if err := v.checkLocked(db); err != nil {
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

	// Clear the passphrase from memory.
	secret.ClearBytes(v.passphrase)
	v.passphrase = nil

	if v.db != nil {
		err := v.db.Close()
		v.db = nil
		return err
	}
	return nil
}

// metadataVerifyKey is the metadata key used to store an encrypted
// verification token. On initialization, a known plaintext is encrypted
// with the passphrase and stored. On subsequent opens, decrypting this
// token verifies that the correct passphrase was provided.
const metadataVerifyKey = "__envref_verify__"

// metadataVerifyPlaintext is the known plaintext encrypted into the
// verification token during vault initialization.
const metadataVerifyPlaintext = "envref-vault-ok"

// ErrVaultNotInitialized is returned when the vault database exists but
// has no verification token — i.e., vault init has not been run.
var ErrVaultNotInitialized = errors.New("vault not initialized: run 'envref vault init'")

// ErrWrongPassphrase is returned when the verification token cannot be
// decrypted with the provided passphrase.
var ErrWrongPassphrase = errors.New("wrong vault passphrase")

// ErrVaultLocked is returned when an operation is attempted on a locked vault.
var ErrVaultLocked = errors.New("vault is locked: run 'envref vault unlock'")

// metadataLockedKey is the metadata key used to persist the vault lock state.
const metadataLockedKey = "__envref_locked__"

// Initialize stores an encrypted verification token in the vault. This
// must be called once before regular use. It returns an error if the
// vault is already initialized.
func (v *VaultBackend) Initialize() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault init: %w", err)
	}

	// Check if already initialized.
	var existing string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataVerifyKey).Scan(&existing)
	if err == nil {
		return fmt.Errorf("vault is already initialized at %s", v.dbPath)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("vault init: checking existing token: %w", err)
	}

	// Encrypt the verification plaintext with the current passphrase.
	token, err := v.encrypt(metadataVerifyPlaintext)
	if err != nil {
		return fmt.Errorf("vault init: encrypting verification token: %w", err)
	}

	_, err = db.Exec("INSERT INTO metadata (key, value) VALUES (?, ?)", metadataVerifyKey, token)
	if err != nil {
		return fmt.Errorf("vault init: storing verification token: %w", err)
	}

	return nil
}

// IsInitialized returns true if the vault has a verification token
// stored (i.e., vault init has been run).
func (v *VaultBackend) IsInitialized() (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return false, fmt.Errorf("vault: %w", err)
	}

	var dummy string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataVerifyKey).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("vault: checking initialization: %w", err)
	}
	return true, nil
}

// VerifyPassphrase checks that the current passphrase can decrypt the
// stored verification token. Returns ErrVaultNotInitialized if no token
// exists, or ErrWrongPassphrase if decryption fails.
func (v *VaultBackend) VerifyPassphrase() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault: %w", err)
	}

	var token string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataVerifyKey).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVaultNotInitialized
	}
	if err != nil {
		return fmt.Errorf("vault: reading verification token: %w", err)
	}

	plaintext, err := v.decrypt(token)
	if err != nil {
		return ErrWrongPassphrase
	}
	if plaintext != metadataVerifyPlaintext {
		return ErrWrongPassphrase
	}

	return nil
}

// IsLocked returns true if the vault has been explicitly locked via the
// lock command. A locked vault refuses all Get/Set/Delete/List operations.
func (v *VaultBackend) IsLocked() (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return false, fmt.Errorf("vault: %w", err)
	}

	var val string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataLockedKey).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("vault: checking lock state: %w", err)
	}
	return val == "true", nil
}

// Lock locks the vault by writing a persistent lock flag to the metadata
// table. The passphrase is verified before locking. A locked vault refuses
// all Get/Set/Delete/List operations until unlocked.
func (v *VaultBackend) Lock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault lock: %w", err)
	}

	// Verify passphrase before locking.
	if err := v.verifyPassphraseUnlocked(db); err != nil {
		return fmt.Errorf("vault lock: %w", err)
	}

	// Check if already locked.
	var existing string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataLockedKey).Scan(&existing)
	if err == nil && existing == "true" {
		return fmt.Errorf("vault is already locked")
	}

	_, err = db.Exec(
		"INSERT INTO metadata (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		metadataLockedKey, "true",
	)
	if err != nil {
		return fmt.Errorf("vault lock: writing lock flag: %w", err)
	}

	return nil
}

// Unlock removes the lock flag from the vault metadata, allowing
// Get/Set/Delete/List operations to proceed again. The passphrase is
// verified before unlocking.
func (v *VaultBackend) Unlock() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return fmt.Errorf("vault unlock: %w", err)
	}

	// Verify passphrase before unlocking.
	if err := v.verifyPassphraseUnlocked(db); err != nil {
		return fmt.Errorf("vault unlock: %w", err)
	}

	// Check if actually locked.
	var existing string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataLockedKey).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && existing != "true") {
		return fmt.Errorf("vault is not locked")
	}
	if err != nil {
		return fmt.Errorf("vault unlock: checking lock state: %w", err)
	}

	_, err = db.Exec("DELETE FROM metadata WHERE key = ?", metadataLockedKey)
	if err != nil {
		return fmt.Errorf("vault unlock: removing lock flag: %w", err)
	}

	return nil
}

// verifyPassphraseUnlocked verifies the passphrase against the stored
// verification token. Must be called with v.mu held. Returns
// ErrVaultNotInitialized or ErrWrongPassphrase on failure.
func (v *VaultBackend) verifyPassphraseUnlocked(db *sql.DB) error {
	var token string
	err := db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataVerifyKey).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrVaultNotInitialized
	}
	if err != nil {
		return fmt.Errorf("reading verification token: %w", err)
	}

	plaintext, err := v.decrypt(token)
	if err != nil {
		return ErrWrongPassphrase
	}
	if plaintext != metadataVerifyPlaintext {
		return ErrWrongPassphrase
	}

	return nil
}

// checkLocked checks whether the vault is locked. Must be called with v.mu held
// and db already opened. Returns ErrVaultLocked if the vault is locked.
func (v *VaultBackend) checkLocked(db *sql.DB) error {
	var val string
	err := db.QueryRow("SELECT value FROM metadata WHERE key = ?", metadataLockedKey).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking lock state: %w", err)
	}
	if val == "true" {
		return ErrVaultLocked
	}
	return nil
}

// open lazily opens (or returns) the SQLite database connection and
// ensures the secrets and metadata tables exist. Must be called with v.mu held.
func (v *VaultBackend) open() (*sql.DB, error) {
	if v.passphrase == nil {
		return nil, ErrVaultClosed
	}
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

	// Create the metadata table for vault state (verification token, etc.).
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY NOT NULL,
		value TEXT NOT NULL
	)`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing vault metadata schema: %w", err)
	}

	v.db = db
	return v.db, nil
}

// ErrVaultClosed is returned when an operation is attempted on a vault whose
// passphrase has been cleared (after Close).
var ErrVaultClosed = errors.New("vault is closed: passphrase has been cleared from memory")

// encrypt encrypts a plaintext string using age scrypt passphrase encryption
// and returns the ASCII-armored ciphertext.
func (v *VaultBackend) encrypt(plaintext string) (string, error) {
	if v.passphrase == nil {
		return "", ErrVaultClosed
	}
	recipient, err := age.NewScryptRecipient(string(v.passphrase))
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
	if v.passphrase == nil {
		return "", ErrVaultClosed
	}
	identity, err := age.NewScryptIdentity(string(v.passphrase))
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

	// Convert to string before clearing the byte slice. The string will
	// hold its own copy; clearing the original byte slice reduces the
	// number of copies of the secret in memory.
	result := string(plaintext)
	secret.ClearBytes(plaintext)

	return result, nil
}

// DBPath returns the path to the vault database file.
func (v *VaultBackend) DBPath() string {
	return v.dbPath
}

// VaultExport represents the JSON structure used for vault export/import.
// It contains all secret key-value pairs in plaintext along with metadata
// about the export.
type VaultExport struct {
	// Version is the export format version for forward compatibility.
	Version int `json:"version"`
	// ExportedAt is the RFC 3339 timestamp when the export was created.
	ExportedAt string `json:"exported_at"`
	// Secrets contains the decrypted key-value pairs.
	Secrets map[string]string `json:"secrets"`
}

// exportVersion is the current export format version.
const exportVersion = 1

// Export decrypts all secrets and returns them as a VaultExport struct.
// Returns ErrVaultLocked if the vault is locked.
func (v *VaultBackend) Export() (*VaultExport, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return nil, fmt.Errorf("vault export: %w", err)
	}

	if err := v.checkLocked(db); err != nil {
		return nil, fmt.Errorf("vault export: %w", err)
	}

	if err := v.verifyPassphraseUnlocked(db); err != nil {
		return nil, fmt.Errorf("vault export: %w", err)
	}

	rows, err := db.Query("SELECT key, value FROM secrets ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("vault export: querying secrets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	secrets := make(map[string]string)
	for rows.Next() {
		var key, encrypted string
		if err := rows.Scan(&key, &encrypted); err != nil {
			return nil, fmt.Errorf("vault export: scanning row: %w", err)
		}
		plaintext, err := v.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("vault export: decrypting %q: %w", key, err)
		}
		secrets[key] = plaintext
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vault export: iterating rows: %w", err)
	}

	return &VaultExport{
		Version:    exportVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Secrets:    secrets,
	}, nil
}

// Import reads a VaultExport and stores all secrets in the vault, encrypting
// each value. Existing keys are overwritten. Returns ErrVaultLocked if the
// vault is locked.
func (v *VaultBackend) Import(export *VaultExport) (int, error) {
	if export == nil {
		return 0, fmt.Errorf("vault import: export data is nil")
	}
	if export.Version != exportVersion {
		return 0, fmt.Errorf("vault import: unsupported export version %d (expected %d)", export.Version, exportVersion)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	db, err := v.open()
	if err != nil {
		return 0, fmt.Errorf("vault import: %w", err)
	}

	if err := v.checkLocked(db); err != nil {
		return 0, fmt.Errorf("vault import: %w", err)
	}

	if err := v.verifyPassphraseUnlocked(db); err != nil {
		return 0, fmt.Errorf("vault import: %w", err)
	}

	count := 0
	for key, value := range export.Secrets {
		encrypted, err := v.encrypt(value)
		if err != nil {
			return count, fmt.Errorf("vault import: encrypting %q: %w", key, err)
		}

		_, err = db.Exec(
			"INSERT INTO secrets (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			key, encrypted,
		)
		if err != nil {
			return count, fmt.Errorf("vault import: storing %q: %w", key, err)
		}
		count++
	}

	return count, nil
}

// ExportJSON is a convenience method that exports the vault and marshals
// it to indented JSON bytes.
func (v *VaultBackend) ExportJSON() ([]byte, error) {
	export, err := v.Export()
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("vault export: marshaling JSON: %w", err)
	}

	return data, nil
}

// ImportJSON is a convenience method that reads JSON bytes and imports
// the secrets into the vault. Returns the number of secrets imported.
func (v *VaultBackend) ImportJSON(data []byte) (int, error) {
	var export VaultExport
	if err := json.Unmarshal(data, &export); err != nil {
		return 0, fmt.Errorf("vault import: parsing JSON: %w", err)
	}

	return v.Import(&export)
}
