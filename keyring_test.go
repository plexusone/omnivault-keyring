package keyring

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/plexusone/omnivault/vault"
	zkeyring "github.com/zalando/go-keyring"
)

func init() {
	// Use mock keyring for all tests
	zkeyring.MockInit()
}

func TestNew(t *testing.T) {
	t.Run("default service name", func(t *testing.T) {
		p := New(Config{})
		if p.ServiceName() != DefaultServiceName {
			t.Errorf("expected service name %q, got %q", DefaultServiceName, p.ServiceName())
		}
	})

	t.Run("custom service name", func(t *testing.T) {
		p := New(Config{ServiceName: "myapp"})
		if p.ServiceName() != "myapp" {
			t.Errorf("expected service name %q, got %q", "myapp", p.ServiceName())
		}
	})
}

func TestNewWithServiceName(t *testing.T) {
	p := NewWithServiceName("testapp")
	if p.ServiceName() != "testapp" {
		t.Errorf("expected service name %q, got %q", "testapp", p.ServiceName())
	}
}

func TestProvider_SetAndGet(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-set-get"})
	defer p.Close()

	t.Run("simple value", func(t *testing.T) {
		err := p.Set(ctx, "key1", &vault.Secret{Value: "value1"})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		secret, err := p.Get(ctx, "key1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if secret.Value != "value1" {
			t.Errorf("expected value %q, got %q", "value1", secret.Value)
		}
		if secret.Metadata.Provider != "keyring" {
			t.Errorf("expected provider %q, got %q", "keyring", secret.Metadata.Provider)
		}
		if secret.Metadata.Path != "key1" {
			t.Errorf("expected path %q, got %q", "key1", secret.Metadata.Path)
		}

		// Cleanup
		_ = p.Delete(ctx, "key1")
	})

	t.Run("path with slashes", func(t *testing.T) {
		err := p.Set(ctx, "database/prod/password", &vault.Secret{Value: "secret123"})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		secret, err := p.Get(ctx, "database/prod/password")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if secret.Value != "secret123" {
			t.Errorf("expected value %q, got %q", "secret123", secret.Value)
		}

		// Cleanup
		_ = p.Delete(ctx, "database/prod/password")
	})
}

func TestProvider_SetAndGet_JSONFormat(t *testing.T) {
	ctx := context.Background()
	p := New(Config{
		ServiceName: "test-json",
		JSONFormat:  true,
	})
	defer p.Close()

	t.Run("multi-field secret", func(t *testing.T) {
		err := p.Set(ctx, "db-creds", &vault.Secret{
			Value: "password123",
			Fields: map[string]string{
				"username": "admin",
				"host":     "localhost",
				"port":     "5432",
			},
		})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		secret, err := p.Get(ctx, "db-creds")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if secret.Value != "password123" {
			t.Errorf("expected value %q, got %q", "password123", secret.Value)
		}
		if secret.Fields["username"] != "admin" {
			t.Errorf("expected username %q, got %q", "admin", secret.Fields["username"])
		}
		if secret.Fields["host"] != "localhost" {
			t.Errorf("expected host %q, got %q", "localhost", secret.Fields["host"])
		}

		// Cleanup
		_ = p.Delete(ctx, "db-creds")
	})
}

func TestProvider_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-notfound"})
	defer p.Close()

	_, err := p.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if !errors.Is(err, vault.ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestProvider_Delete(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-delete"})
	defer p.Close()

	// Set a secret
	err := p.Set(ctx, "to-delete", &vault.Secret{Value: "temp"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	exists, err := p.Exists(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected secret to exist")
	}

	// Delete it
	err = p.Delete(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	exists, err = p.Exists(ctx, "to-delete")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected secret to not exist after deletion")
	}

	// Delete nonexistent should not error
	err = p.Delete(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Delete of nonexistent key should not error, got %v", err)
	}
}

func TestProvider_Exists(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-exists"})
	defer p.Close()

	// Nonexistent key
	exists, err := p.Exists(ctx, "missing")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected nonexistent key to return false")
	}

	// Create and check
	err = p.Set(ctx, "present", &vault.Secret{Value: "here"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	defer func() { _ = p.Delete(ctx, "present") }()

	exists, err = p.Exists(ctx, "present")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected existing key to return true")
	}
}

func TestProvider_List(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-list"})
	defer p.Close()

	// Set up test data
	secrets := []string{
		"database/prod",
		"database/staging",
		"api/github",
		"api/stripe",
		"standalone",
	}
	for _, key := range secrets {
		err := p.Set(ctx, key, &vault.Secret{Value: "test"})
		if err != nil {
			t.Fatalf("Set failed for %s: %v", key, err)
		}
	}
	defer func() {
		for _, key := range secrets {
			_ = p.Delete(ctx, key)
		}
	}()

	t.Run("list all", func(t *testing.T) {
		list, err := p.List(ctx, "")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != len(secrets) {
			t.Errorf("expected %d items, got %d", len(secrets), len(list))
		}
	})

	t.Run("list with prefix", func(t *testing.T) {
		list, err := p.List(ctx, "database/")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("expected 2 items with prefix 'database/', got %d", len(list))
		}

		list, err = p.List(ctx, "api/")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("expected 2 items with prefix 'api/', got %d", len(list))
		}
	})

	t.Run("list with no matches", func(t *testing.T) {
		list, err := p.List(ctx, "nonexistent/")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected 0 items, got %d", len(list))
		}
	})
}

func TestProvider_Name(t *testing.T) {
	p := New(Config{})
	if p.Name() != "keyring" {
		t.Errorf("expected name %q, got %q", "keyring", p.Name())
	}
}

func TestProvider_Capabilities(t *testing.T) {
	t.Run("without JSON format", func(t *testing.T) {
		p := New(Config{})
		caps := p.Capabilities()
		if !caps.Read || !caps.Write || !caps.Delete || !caps.List {
			t.Error("expected all basic capabilities to be true")
		}
		if caps.MultiField {
			t.Error("expected MultiField to be false without JSONFormat")
		}
	})

	t.Run("with JSON format", func(t *testing.T) {
		p := New(Config{JSONFormat: true})
		caps := p.Capabilities()
		if !caps.MultiField {
			t.Error("expected MultiField to be true with JSONFormat")
		}
	})
}

func TestProvider_Backend(t *testing.T) {
	p := New(Config{})
	backend := p.Backend()
	// Just verify it returns something reasonable
	if backend == "" {
		t.Error("expected non-empty backend name")
	}
}

func TestProvider_Close(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-close"})

	// Operations should work before close
	err := p.Set(ctx, "test", &vault.Secret{Value: "value"})
	if err != nil {
		t.Fatalf("Set failed before close: %v", err)
	}

	// Close the provider
	err = p.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations should fail after close
	_, err = p.Get(ctx, "test")
	if !errors.Is(err, vault.ErrClosed) {
		t.Errorf("expected ErrClosed after close, got %v", err)
	}

	err = p.Set(ctx, "test2", &vault.Secret{Value: "value"})
	if !errors.Is(err, vault.ErrClosed) {
		t.Errorf("expected ErrClosed after close, got %v", err)
	}

	err = p.Delete(ctx, "test")
	if !errors.Is(err, vault.ErrClosed) {
		t.Errorf("expected ErrClosed after close, got %v", err)
	}

	_, err = p.Exists(ctx, "test")
	if !errors.Is(err, vault.ErrClosed) {
		t.Errorf("expected ErrClosed after close, got %v", err)
	}

	_, err = p.List(ctx, "")
	if !errors.Is(err, vault.ErrClosed) {
		t.Errorf("expected ErrClosed after close, got %v", err)
	}
}

func TestProvider_OnIndexError(t *testing.T) {
	ctx := context.Background()

	var capturedOp string
	var capturedErr error
	var mu sync.Mutex

	p := New(Config{
		ServiceName: "test-index-error",
		OnIndexError: func(op string, err error) {
			mu.Lock()
			defer mu.Unlock()
			capturedOp = op
			capturedErr = err
		},
	})
	defer p.Close()

	// Normal operations should not trigger the callback
	err := p.Set(ctx, "normal-key", &vault.Secret{Value: "value"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	mu.Lock()
	if capturedErr != nil {
		t.Errorf("unexpected error callback: op=%s err=%v", capturedOp, capturedErr)
	}
	mu.Unlock()

	// Cleanup
	_ = p.Delete(ctx, "normal-key")
}

func TestProvider_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	p := New(Config{ServiceName: "test-concurrent"})
	defer p.Close()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := "concurrent-key"
			err := p.Set(ctx, key, &vault.Secret{Value: "value"})
			if err != nil {
				errCh <- err
			}
		}()
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = p.Get(ctx, "concurrent-key")
			_, _ = p.Exists(ctx, "concurrent-key")
			_, _ = p.List(ctx, "")
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation failed: %v", err)
	}

	// Cleanup
	_ = p.Delete(ctx, "concurrent-key")
}

func TestProvider_ImplementsVaultInterface(t *testing.T) {
	var _ vault.Vault = (*Provider)(nil)
}
