package osbridge

import (
	"errors"
	"fmt"
	"sync"
)

const (
	maxArchiveEntryBytes   = 100 << 20
	errWasmKeyNotFoundFmt  = "wasm storage key not found: %s"
	errWasmOffsetBoundsFmt = "wasm storage write offset out of bounds: %d"
	errWasmFetchDisabled   = "network fetch is not supported in WasmStorage"
	errWasmArchiveDisabled = "zip archive decoding is not supported in WasmStorage"
	blobURLPrefix          = "blob:mojra/"
)

// Storage defines the domain-agnostic byte, archive, blob, and network storage contract.
type Storage interface {
	ReadBytes(location string) ([]byte, error)
	WriteAtomic(location string, payload []byte) error
	WriteAt(location string, payload []byte, offset int64) error
	ReadArchive(location string) (map[string][]byte, error)
	CreateBlobURL(location string, payload []byte, mimeType string) string
	FetchURLBytes(rawURL string, maxBytes int64) ([]byte, string, error)
}

// WasmStorage implements Storage in memory with optional browser localStorage and Blob URL hooks.
type WasmStorage struct {
	mu          sync.RWMutex
	files       map[string][]byte
	blobURLs    map[string]string
	LoadKVHook  func(key string) (string, bool)
	StoreKVHook func(key string, value string)
	BlobURLHook func(location string, payload []byte, mimeType string) string
}

// NewWasmStorage constructs an in-memory Storage implementation suitable for WebAssembly and isolated tests.
func NewWasmStorage() *WasmStorage {
	return &WasmStorage{
		files:    make(map[string][]byte),
		blobURLs: make(map[string]string),
	}
}

// ReadBytes reads raw bytes from memory or the configured KV hook.
func (storage *WasmStorage) ReadBytes(location string) ([]byte, error) {
	storage.mu.RLock()
	existing, found := storage.files[location]
	storage.mu.RUnlock()
	if found {
		cloned := make([]byte, len(existing))
		copy(cloned, existing)
		return cloned, nil
	}
	if storage.LoadKVHook != nil {
		if rawVal, ok := storage.LoadKVHook(location); ok {
			return []byte(rawVal), nil
		}
	}
	return nil, fmt.Errorf(errWasmKeyNotFoundFmt, location)
}

func (storage *WasmStorage) putFileBytes(location string, cloned []byte) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.files[location] = cloned
}

// WriteAtomic stores payload bytes in memory and persists them through StoreKVHook if configured.
func (storage *WasmStorage) WriteAtomic(location string, payload []byte) error {
	cloned := make([]byte, len(payload))
	copy(cloned, payload)
	storage.putFileBytes(location, cloned)
	if storage.StoreKVHook != nil {
		storage.StoreKVHook(location, string(cloned))
	}
	return nil
}

func (storage *WasmStorage) patchFileBytes(location string, payload []byte, offset int64) (string, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	existing, found := storage.files[location]
	if !found {
		return "", fmt.Errorf(errWasmKeyNotFoundFmt, location)
	}
	endPos := int(offset) + len(payload)
	if endPos > len(existing) {
		expanded := make([]byte, endPos)
		copy(expanded, existing)
		existing = expanded
	}
	copy(existing[int(offset):endPos], payload)
	storage.files[location] = existing
	return string(existing), nil
}

// WriteAt patches payload bytes at offset inside an existing in-memory entry.
func (storage *WasmStorage) WriteAt(location string, payload []byte, offset int64) error {
	if offset < 0 {
		return fmt.Errorf(errWasmOffsetBoundsFmt, offset)
	}
	updatedCopy, patchErr := storage.patchFileBytes(location, payload, offset)
	if patchErr != nil {
		return patchErr
	}
	if storage.StoreKVHook != nil {
		storage.StoreKVHook(location, updatedCopy)
	}
	return nil
}

// ReadArchive returns an error in WasmStorage because WASM uses STBC binary containers rather than zip archives.
func (storage *WasmStorage) ReadArchive(_ string) (map[string][]byte, error) {
	return nil, errors.New(errWasmArchiveDisabled)
}

func (storage *WasmStorage) storeBlobURL(location, resolvedURL string) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.blobURLs[location] = resolvedURL
}

// CreateBlobURL creates or returns a cached Blob URL for a media payload.
func (storage *WasmStorage) CreateBlobURL(location string, payload []byte, mimeType string) string {
	storage.mu.RLock()
	cachedURL, found := storage.blobURLs[location]
	storage.mu.RUnlock()
	if found {
		return cachedURL
	}
	resolvedURL := blobURLPrefix + location
	if storage.BlobURLHook != nil {
		if hooked := storage.BlobURLHook(location, payload, mimeType); hooked != "" {
			resolvedURL = hooked
		}
	}
	storage.storeBlobURL(location, resolvedURL)
	return resolvedURL
}

// FetchURLBytes returns an error because direct outbound network fetches are disabled in WasmStorage.
func (storage *WasmStorage) FetchURLBytes(_ string, _ int64) ([]byte, string, error) {
	return nil, "", errors.New(errWasmFetchDisabled)
}
