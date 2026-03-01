// Package keyring provides a cross-platform vault implementation using OS credential stores.
//
// Supported backends:
//   - macOS: Keychain
//   - Windows: Credential Manager
//   - Linux: Secret Service (GNOME Keyring, KWallet)
//
// Usage:
//
//	v := keyring.New(keyring.Config{
//	    ServiceName: "myapp",
//	})
//	err := v.Set(ctx, "api-key", &vault.Secret{Value: "secret123"})
//	secret, err := v.Get(ctx, "api-key")
//
// With omnivault client:
//
//	client, _ := omnivault.NewClient(omnivault.Config{
//	    CustomVault: keyring.New(keyring.Config{ServiceName: "myapp"}),
//	})
//
// With resolver:
//
//	resolver := omnivault.NewResolver()
//	resolver.Register("keyring", keyring.New(keyring.Config{ServiceName: "myapp"}))
//	value, _ := resolver.Resolve(ctx, "keyring://api-key")
package keyring

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"sync"

	"github.com/plexusone/omnivault/vault"
	zkeyring "github.com/zalando/go-keyring"
)

const (
	// DefaultServiceName is the default service name used if none is provided.
	DefaultServiceName = "omnivault"

	// indexKey is the key used to store the list of all secret keys.
	// This enables the List() functionality since OS keyrings don't support enumeration.
	indexKey = "__omnivault_index__"
)

// Config holds configuration for the keyring provider.
type Config struct {
	// ServiceName is the service/application name used to namespace secrets.
	// On macOS, this appears as the "Where" field in Keychain Access.
	// On Windows, this is part of the target name.
	// Default: "omnivault"
	ServiceName string

	// JSONFormat stores secrets as JSON with metadata support.
	// When false, only simple string values are stored.
	// Default: false
	JSONFormat bool

	// OnIndexError is called when an error occurs during index operations.
	// Index operations are used to track stored keys for List() functionality.
	// These errors are non-fatal (Get/Set/Delete still work) but may cause
	// List() to return incomplete results.
	// If nil, index errors are silently ignored.
	OnIndexError func(op string, err error)
}

// Provider implements vault.Vault using OS credential stores.
type Provider struct {
	config Config
	mu     sync.RWMutex
	closed bool
}

// New creates a new keyring provider with the given configuration.
func New(config Config) *Provider {
	if config.ServiceName == "" {
		config.ServiceName = DefaultServiceName
	}
	return &Provider{config: config}
}

// NewWithServiceName creates a new keyring provider with the specified service name.
func NewWithServiceName(serviceName string) *Provider {
	return New(Config{ServiceName: serviceName})
}

// Get retrieves a secret from the OS keyring.
func (p *Provider) Get(ctx context.Context, path string) (*vault.Secret, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, vault.NewVaultError("Get", path, p.Name(), vault.ErrClosed)
	}

	value, err := zkeyring.Get(p.config.ServiceName, path)
	if err != nil {
		if errors.Is(err, zkeyring.ErrNotFound) {
			return nil, vault.NewVaultError("Get", path, p.Name(), vault.ErrSecretNotFound)
		}
		return nil, vault.NewVaultError("Get", path, p.Name(), err)
	}

	secret := &vault.Secret{
		Metadata: vault.Metadata{
			Provider: p.Name(),
			Path:     path,
		},
	}

	if p.config.JSONFormat {
		if err := json.Unmarshal([]byte(value), secret); err != nil {
			// Fall back to plain value if JSON parsing fails
			secret.Value = value
		}
	} else {
		secret.Value = value
	}

	return secret, nil
}

// Set stores a secret in the OS keyring.
func (p *Provider) Set(ctx context.Context, path string, secret *vault.Secret) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return vault.NewVaultError("Set", path, p.Name(), vault.ErrClosed)
	}

	var value string
	if p.config.JSONFormat {
		data, err := json.Marshal(secret)
		if err != nil {
			return vault.NewVaultError("Set", path, p.Name(), err)
		}
		value = string(data)
	} else {
		value = secret.String()
	}

	if err := zkeyring.Set(p.config.ServiceName, path, value); err != nil {
		return vault.NewVaultError("Set", path, p.Name(), err)
	}

	// Update the index for List() support
	if path != indexKey {
		p.addToIndex(path)
	}

	return nil
}

// Delete removes a secret from the OS keyring.
func (p *Provider) Delete(ctx context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return vault.NewVaultError("Delete", path, p.Name(), vault.ErrClosed)
	}

	if err := zkeyring.Delete(p.config.ServiceName, path); err != nil {
		if errors.Is(err, zkeyring.ErrNotFound) {
			return nil // Already deleted
		}
		return vault.NewVaultError("Delete", path, p.Name(), err)
	}

	// Update the index
	if path != indexKey {
		p.removeFromIndex(path)
	}

	return nil
}

// Exists checks if a secret exists in the OS keyring.
func (p *Provider) Exists(ctx context.Context, path string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return false, vault.NewVaultError("Exists", path, p.Name(), vault.ErrClosed)
	}

	_, err := zkeyring.Get(p.config.ServiceName, path)
	if err != nil {
		if errors.Is(err, zkeyring.ErrNotFound) {
			return false, nil
		}
		return false, vault.NewVaultError("Exists", path, p.Name(), err)
	}
	return true, nil
}

// List returns all secret paths matching the prefix.
// Note: OS keyrings don't natively support enumeration, so this relies on
// an internal index that tracks stored keys.
func (p *Provider) List(ctx context.Context, prefix string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, vault.NewVaultError("List", prefix, p.Name(), vault.ErrClosed)
	}

	index := p.loadIndex()
	var results []string
	for _, key := range index {
		if strings.HasPrefix(key, prefix) {
			results = append(results, key)
		}
	}
	return results, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "keyring"
}

// Capabilities returns the provider capabilities.
func (p *Provider) Capabilities() vault.Capabilities {
	return vault.Capabilities{
		Read:       true,
		Write:      true,
		Delete:     true,
		List:       true, // Via internal index
		MultiField: p.config.JSONFormat,
	}
}

// Close marks the provider as closed.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

// ServiceName returns the configured service name.
func (p *Provider) ServiceName() string {
	return p.config.ServiceName
}

// Backend returns the name of the OS keyring backend being used.
func (p *Provider) Backend() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS Keychain"
	case "windows":
		return "Windows Credential Manager"
	case "linux":
		return "Secret Service (GNOME Keyring/KWallet)"
	default:
		return "Unknown"
	}
}

// loadIndex loads the list of stored keys from the index.
func (p *Provider) loadIndex() []string {
	value, err := zkeyring.Get(p.config.ServiceName, indexKey)
	if err != nil {
		// Only report non-"not found" errors (index may not exist yet)
		if !errors.Is(err, zkeyring.ErrNotFound) {
			p.reportIndexError("load", err)
		}
		return nil
	}
	var index []string
	if err := json.Unmarshal([]byte(value), &index); err != nil {
		p.reportIndexError("unmarshal", err)
		return nil
	}
	return index
}

// saveIndex saves the list of stored keys to the index.
func (p *Provider) saveIndex(index []string) {
	data, err := json.Marshal(index)
	if err != nil {
		p.reportIndexError("marshal", err)
		return
	}
	if err := zkeyring.Set(p.config.ServiceName, indexKey, string(data)); err != nil {
		p.reportIndexError("save", err)
	}
}

// reportIndexError calls the OnIndexError callback if configured.
func (p *Provider) reportIndexError(op string, err error) {
	if p.config.OnIndexError != nil {
		p.config.OnIndexError(op, err)
	}
}

// addToIndex adds a key to the index.
func (p *Provider) addToIndex(key string) {
	index := p.loadIndex()
	for _, k := range index {
		if k == key {
			return // Already in index
		}
	}
	index = append(index, key)
	p.saveIndex(index)
}

// removeFromIndex removes a key from the index.
func (p *Provider) removeFromIndex(key string) {
	index := p.loadIndex()
	newIndex := make([]string, 0, len(index))
	for _, k := range index {
		if k != key {
			newIndex = append(newIndex, k)
		}
	}
	p.saveIndex(newIndex)
}

// Ensure Provider implements vault.Vault.
var _ vault.Vault = (*Provider)(nil)
