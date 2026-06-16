package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	officialCredentialsKey = "official_api.credentials"
	officialCredentialsAlg = "AES-256-GCM"
)

type OfficialCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type OfficialCredentialStatus struct {
	Saved          bool   `json:"saved"`
	Usable         bool   `json:"usable"`
	MaskedUsername string `json:"maskedUsername,omitempty"`
	Message        string `json:"message,omitempty"`
}

type officialCredentialRecord struct {
	Algorithm      string `json:"algorithm"`
	Nonce          string `json:"nonce"`
	Ciphertext     string `json:"ciphertext"`
	MaskedUsername string `json:"maskedUsername,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type OfficialCredentialVault struct {
	keyPath string
	kvPath  string
	mu      sync.Mutex
}

func NewOfficialCredentialVault(keyPath string, kvPath string) (*OfficialCredentialVault, error) {
	keyPath = strings.TrimSpace(keyPath)
	kvPath = strings.TrimSpace(kvPath)
	if keyPath == "" || kvPath == "" {
		return nil, errors.New("official credential key and kv paths are required")
	}
	vault := &OfficialCredentialVault{keyPath: keyPath, kvPath: kvPath}
	if _, err := vault.loadOrCreateKey(); err != nil {
		return nil, err
	}
	return vault, nil
}

func (v *OfficialCredentialVault) Status(ctx context.Context) (OfficialCredentialStatus, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.statusLocked(ctx)
}

func (v *OfficialCredentialVault) Save(ctx context.Context, credentials OfficialCredentials) error {
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || credentials.Password == "" {
		return errors.New("official API username and password are required")
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	key, err := v.loadOrCreateKey()
	if err != nil {
		return err
	}
	aead, err := aesGCM(key)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate credential nonce: %w", err)
	}
	plain, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("marshal official credentials: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, plain, []byte(officialCredentialsKey))

	store, err := readKVFile(v.kvPath)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := now
	if raw := store[officialCredentialsKey]; len(raw) > 0 {
		var existing officialCredentialRecord
		if json.Unmarshal(raw, &existing) == nil && existing.CreatedAt != "" {
			createdAt = existing.CreatedAt
		}
	}
	record := officialCredentialRecord{
		Algorithm:      officialCredentialsAlg,
		Nonce:          base64.StdEncoding.EncodeToString(nonce),
		Ciphertext:     base64.StdEncoding.EncodeToString(ciphertext),
		MaskedUsername: maskUsername(credentials.Username),
		CreatedAt:      createdAt,
		UpdatedAt:      now,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal official credential record: %w", err)
	}
	store[officialCredentialsKey] = raw
	return writeKVFile(v.kvPath, store)
}

func (v *OfficialCredentialVault) Load(ctx context.Context) (OfficialCredentials, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.loadLocked(ctx)
}

func (v *OfficialCredentialVault) Delete(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	store, err := readKVFile(v.kvPath)
	if err != nil {
		return err
	}
	delete(store, officialCredentialsKey)
	return writeKVFile(v.kvPath, store)
}

func (v *OfficialCredentialVault) statusLocked(ctx context.Context) (OfficialCredentialStatus, error) {
	store, err := readKVFile(v.kvPath)
	if err != nil {
		return OfficialCredentialStatus{Saved: false, Usable: false, Message: "settings unavailable"}, err
	}
	raw := store[officialCredentialsKey]
	if len(raw) == 0 {
		return OfficialCredentialStatus{Saved: false, Usable: false, Message: "no saved official API credentials"}, nil
	}
	var record officialCredentialRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return OfficialCredentialStatus{Saved: true, Usable: false, Message: "settings unavailable"}, sanitizedSettingsError(err)
	}
	if _, err := v.loadLocked(ctx); err != nil {
		return OfficialCredentialStatus{Saved: true, Usable: false, MaskedUsername: record.MaskedUsername, Message: "saved official API credentials are unavailable"}, err
	}
	return OfficialCredentialStatus{Saved: true, Usable: true, MaskedUsername: record.MaskedUsername, Message: "saved official API credentials are ready"}, nil
}

func (v *OfficialCredentialVault) loadLocked(ctx context.Context) (OfficialCredentials, error) {
	store, err := readKVFile(v.kvPath)
	if err != nil {
		return OfficialCredentials{}, err
	}
	raw := store[officialCredentialsKey]
	if len(raw) == 0 {
		return OfficialCredentials{}, errors.New("no saved official API credentials")
	}
	var record officialCredentialRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return OfficialCredentials{}, sanitizedSettingsError(err)
	}
	if record.Algorithm != officialCredentialsAlg {
		return OfficialCredentials{}, errors.New("saved official API credentials use an unsupported encryption algorithm")
	}
	key, err := v.loadOrCreateKey()
	if err != nil {
		return OfficialCredentials{}, err
	}
	aead, err := aesGCM(key)
	if err != nil {
		return OfficialCredentials{}, err
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return OfficialCredentials{}, errors.New("saved official API credentials are unavailable")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return OfficialCredentials{}, errors.New("saved official API credentials are unavailable")
	}
	plain, err := aead.Open(nil, nonce, ciphertext, []byte(officialCredentialsKey))
	if err != nil {
		return OfficialCredentials{}, errors.New("saved official API credentials are unavailable")
	}
	var credentials OfficialCredentials
	if err := json.Unmarshal(plain, &credentials); err != nil {
		return OfficialCredentials{}, errors.New("saved official API credentials are unavailable")
	}
	if strings.TrimSpace(credentials.Username) == "" || credentials.Password == "" {
		return OfficialCredentials{}, errors.New("saved official API credentials are incomplete")
	}
	return credentials, nil
}

func (v *OfficialCredentialVault) loadOrCreateKey() ([]byte, error) {
	if key, err := readAppKey(v.keyPath); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate official API app key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(v.keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("create app key directory: %w", err)
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(key) + "\n")
	if err := os.WriteFile(v.keyPath, encoded, 0o600); err != nil {
		return nil, fmt.Errorf("write official API app key: %w", err)
	}
	return key, nil
}

func readAppKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("official API app key is unavailable")
	}
	return key, nil
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create official API credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create official API credential AEAD: %w", err)
	}
	return aead, nil
}

func readKVFile(path string) (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, sanitizedSettingsError(err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var store map[string]json.RawMessage
	if err := json.Unmarshal(raw, &store); err != nil {
		return nil, sanitizedSettingsError(err)
	}
	if store == nil {
		store = map[string]json.RawMessage{}
	}
	return store, nil
}

func writeKVFile(path string, store map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings kv: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".loinc-browser-kv-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary settings kv: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary settings kv: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary settings kv permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary settings kv: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace settings kv: %w", err)
	}
	return nil
}

func sanitizedSettingsError(err error) error {
	return fmt.Errorf("settings unavailable: %w", err)
}

func maskUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	if at := strings.Index(username, "@"); at > 0 {
		local := username[:at]
		domain := username[at:]
		if len(local) <= 2 {
			return local[:1] + "*" + domain
		}
		return local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:] + domain
	}
	if len(username) <= 2 {
		return username[:1] + "*"
	}
	return username[:1] + strings.Repeat("*", len(username)-2) + username[len(username)-1:]
}
