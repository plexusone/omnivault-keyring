# OmniVault Keyring

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![License][license-svg]][license-url]

Cross-platform OS credential store provider for [OmniVault](https://github.com/plexusone/omnivault). Store and retrieve secrets securely using your operating system's native credential manager.

## Features

- 🌐 **Cross-Platform**: Works on macOS, Windows, and Linux
- 🔐 **Native Security**: Uses OS-provided secure storage (not custom encryption)
- 📦 **Multi-Field Secrets**: Store complex credentials with multiple fields (username, password, host, etc.)
- 🔌 **OmniVault Compatible**: Implements the `vault.Vault` interface for seamless integration
- 🔗 **URI Resolution**: Use `keyring://` URIs with OmniVault's resolver
- 📋 **Listing Support**: Enumerate stored secrets (via internal index)
- ⚡ **Zero Configuration**: Works out of the box with sensible defaults

## Supported Platforms

| Platform | Backend | Storage Location |
|----------|---------|------------------|
| **macOS** | Keychain | `~/Library/Keychains/login.keychain-db` |
| **Windows** | Credential Manager | Windows Credential Locker |
| **Linux** | Secret Service API | GNOME Keyring or KWallet |

## Installation

```bash
go get github.com/plexusone/omnivault-keyring
```

### Linux Prerequisites

On Linux, you need a Secret Service implementation:

```bash
# Debian/Ubuntu (GNOME Keyring)
sudo apt-get install gnome-keyring libsecret-1-0

# Fedora
sudo dnf install gnome-keyring libsecret

# Arch Linux
sudo pacman -S gnome-keyring libsecret

# For headless servers, you may need to start the daemon:
eval $(gnome-keyring-daemon --start --components=secrets)
export GNOME_KEYRING_CONTROL
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/plexusone/omnivault-keyring"
    "github.com/plexusone/omnivault/vault"
)

func main() {
    ctx := context.Background()

    // Create a keyring provider
    kr := keyring.New(keyring.Config{
        ServiceName: "myapp",
    })

    // Store a secret
    err := kr.Set(ctx, "api-key", &vault.Secret{Value: "sk-12345"})
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve the secret
    secret, err := kr.Get(ctx, "api-key")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("API Key:", secret.Value)

    // Clean up
    kr.Delete(ctx, "api-key")
}
```

## Usage Examples

### Basic Secret Storage

```go
import (
    "github.com/plexusone/omnivault-keyring"
    "github.com/plexusone/omnivault/vault"
)

// Create provider with service name
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
})

// Or use the shorthand
kr := keyring.NewWithServiceName("myapp")

// Store a simple secret
kr.Set(ctx, "api-key", &vault.Secret{Value: "secret-value"})

// Retrieve
secret, err := kr.Get(ctx, "api-key")
if err != nil {
    if errors.Is(err, vault.ErrSecretNotFound) {
        // Handle missing secret
    }
}
fmt.Println(secret.Value)

// Check existence
exists, _ := kr.Exists(ctx, "api-key")

// Delete
kr.Delete(ctx, "api-key")
```

### Multi-Field Secrets (JSON Format)

Store complex credentials with multiple fields like database connections, OAuth tokens, or API configurations:

```go
// Enable JSON format for multi-field support
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
    JSONFormat:  true,
})

// Store database credentials
err := kr.Set(ctx, "database/production", &vault.Secret{
    Value: "super-secret-password",  // Primary value (the password)
    Fields: map[string]string{
        "username": "db_admin",
        "host":     "db.example.com",
        "port":     "5432",
        "database": "myapp_prod",
        "sslmode":  "require",
    },
    Metadata: vault.Metadata{
        Tags: map[string]string{
            "environment": "production",
            "team":        "backend",
        },
    },
})

// Retrieve and access fields
secret, _ := kr.Get(ctx, "database/production")
fmt.Printf("Host: %s\n", secret.Fields["host"])
fmt.Printf("Port: %s\n", secret.Fields["port"])
fmt.Printf("User: %s\n", secret.Fields["username"])
fmt.Printf("Pass: %s\n", secret.Value)

// Build a connection string
connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
    secret.Fields["username"],
    secret.Value,
    secret.Fields["host"],
    secret.Fields["port"],
    secret.Fields["database"],
    secret.Fields["sslmode"],
)
```

### OAuth Token Storage

```go
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
    JSONFormat:  true,
})

// Store OAuth tokens
kr.Set(ctx, "oauth/github", &vault.Secret{
    Value: "gho_xxxxxxxxxxxx",  // Access token
    Fields: map[string]string{
        "refresh_token": "ghr_xxxxxxxxxxxx",
        "token_type":    "bearer",
        "scope":         "repo,user",
        "expires_at":    "2024-12-31T23:59:59Z",
    },
})

// Retrieve tokens
secret, _ := kr.Get(ctx, "oauth/github")
accessToken := secret.Value
refreshToken := secret.Fields["refresh_token"]
```

### Integration with OmniVault Client

Use keyring as a backend for the OmniVault client:

```go
import (
    "github.com/plexusone/omnivault"
    "github.com/plexusone/omnivault-keyring"
)

// Create keyring provider
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
    JSONFormat:  true,
})

// Create OmniVault client with keyring backend
client, err := omnivault.NewClient(omnivault.Config{
    CustomVault: kr,
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Use the standard OmniVault API
client.SetValue(ctx, "api/token", "secret-token")
token, _ := client.GetValue(ctx, "api/token")

// Get specific field from a multi-field secret
username, _ := client.GetField(ctx, "database/prod", "username")
```

### Multi-Provider Setup with Resolver

Combine keyring with other providers using OmniVault's resolver:

```go
import (
    "github.com/plexusone/omnivault"
    "github.com/plexusone/omnivault-keyring"
    "github.com/plexusone/omnivault/providers/env"
    "github.com/plexusone/omnivault/providers/file"
)

// Create resolver
resolver := omnivault.NewResolver()

// Register multiple providers
resolver.Register("keyring", keyring.New(keyring.Config{
    ServiceName: "myapp",
}))
resolver.Register("env", env.New())

fileVault, _ := file.New(file.Config{
    Directory: "/etc/myapp/secrets",
})
resolver.Register("file", fileVault)

// Resolve secrets from different sources using URIs
dbPassword, _ := resolver.Resolve(ctx, "keyring://database/password")
apiKey, _ := resolver.Resolve(ctx, "env://API_KEY")
certData, _ := resolver.Resolve(ctx, "file://tls/cert.pem")

// Resolve conditionally - returns value as-is if not a secret URI
config := map[string]string{
    "database_url":  "keyring://database/url",
    "api_key":       "env://API_KEY",
    "log_level":     "info",  // Not a secret URI, returned as-is
}
resolved, _ := resolver.ResolveMap(ctx, config)
```

### Listing and Enumerating Secrets

```go
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
})

// Store some secrets
kr.Set(ctx, "database/prod", &vault.Secret{Value: "pass1"})
kr.Set(ctx, "database/staging", &vault.Secret{Value: "pass2"})
kr.Set(ctx, "api/github", &vault.Secret{Value: "token1"})
kr.Set(ctx, "api/stripe", &vault.Secret{Value: "token2"})

// List all secrets
all, _ := kr.List(ctx, "")
// Returns: ["database/prod", "database/staging", "api/github", "api/stripe"]

// List secrets by prefix
dbSecrets, _ := kr.List(ctx, "database/")
// Returns: ["database/prod", "database/staging"]

apiSecrets, _ := kr.List(ctx, "api/")
// Returns: ["api/github", "api/stripe"]
```

### Application Configuration Pattern

A common pattern for application secrets:

```go
package config

import (
    "context"
    "os"

    "github.com/plexusone/omnivault"
    "github.com/plexusone/omnivault-keyring"
    "github.com/plexusone/omnivault/providers/env"
)

type Config struct {
    DatabaseURL string
    APIKey      string
    JWTSecret   string
}

func LoadConfig(ctx context.Context) (*Config, error) {
    // Create resolver with keyring for secrets, env for non-sensitive config
    resolver := omnivault.NewResolver()
    resolver.Register("keyring", keyring.NewWithServiceName("myapp"))
    resolver.Register("env", env.New())

    // In development, secrets might be in env vars
    // In production, they're in the system keyring
    var secretScheme string
    if os.Getenv("ENV") == "production" {
        secretScheme = "keyring"
    } else {
        secretScheme = "env"
    }

    dbURL, err := resolver.Resolve(ctx, secretScheme+"://DATABASE_URL")
    if err != nil {
        return nil, err
    }

    apiKey, err := resolver.Resolve(ctx, secretScheme+"://API_KEY")
    if err != nil {
        return nil, err
    }

    jwtSecret, err := resolver.Resolve(ctx, secretScheme+"://JWT_SECRET")
    if err != nil {
        return nil, err
    }

    return &Config{
        DatabaseURL: dbURL,
        APIKey:      apiKey,
        JWTSecret:   jwtSecret,
    }, nil
}
```

## Configuration Reference

```go
type Config struct {
    // ServiceName is the service/application name used to namespace secrets.
    //
    // On macOS: Appears as the "Where" field in Keychain Access
    // On Windows: Part of the target name in Credential Manager
    // On Linux: The "application" attribute in Secret Service
    //
    // Default: "omnivault"
    // Recommended: Use your application name (e.g., "myapp", "acme-api")
    ServiceName string

    // JSONFormat enables storing secrets as JSON objects.
    //
    // When true:
    //   - Secrets are serialized as JSON
    //   - Multi-field secrets (Fields map) are supported
    //   - Metadata is preserved
    //
    // When false:
    //   - Only the Value field is stored as plain text
    //   - More compatible with external tools reading the keyring
    //
    // Default: false
    JSONFormat bool
}
```

## API Reference

### Constructor Functions

```go
// New creates a provider with full configuration
func New(config Config) *Provider

// NewWithServiceName creates a provider with just a service name
func NewWithServiceName(serviceName string) *Provider
```

### Vault Interface Methods

```go
// Get retrieves a secret by path
func (p *Provider) Get(ctx context.Context, path string) (*vault.Secret, error)

// Set stores a secret at the given path
func (p *Provider) Set(ctx context.Context, path string, secret *vault.Secret) error

// Delete removes a secret
func (p *Provider) Delete(ctx context.Context, path string) error

// Exists checks if a secret exists
func (p *Provider) Exists(ctx context.Context, path string) (bool, error)

// List returns all paths matching the prefix
func (p *Provider) List(ctx context.Context, prefix string) ([]string, error)

// Name returns "keyring"
func (p *Provider) Name() string

// Capabilities returns supported operations
func (p *Provider) Capabilities() vault.Capabilities

// Close releases resources
func (p *Provider) Close() error
```

### Provider-Specific Methods

```go
// ServiceName returns the configured service name
func (p *Provider) ServiceName() string

// Backend returns the OS backend name
// Returns: "macOS Keychain", "Windows Credential Manager",
//          or "Secret Service (GNOME Keyring/KWallet)"
func (p *Provider) Backend() string
```

## URI Scheme

When using with OmniVault's resolver, use the `keyring://` scheme:

```
keyring://path/to/secret[#field]

Examples:
  keyring://api-key                    → Get api-key secret
  keyring://database/production        → Get database/production secret
  keyring://oauth/github#refresh_token → Get refresh_token field from oauth/github
```

## Platform-Specific Notes

### macOS

**Viewing Secrets:**
1. Open **Keychain Access** (`/Applications/Utilities/Keychain Access.app`)
2. Select **login** keychain in the sidebar
3. Search for your service name
4. Double-click an entry and check "Show password"

**Security:**
- First access prompts for keychain password
- Apps may request "Always Allow" access
- Secrets are encrypted with the user's login password

**Code Signing:**
- Unsigned apps may trigger additional security prompts
- For production, sign your application with a Developer ID

### Windows

**Viewing Secrets:**
1. Open **Control Panel** → **User Accounts** → **Credential Manager**
2. Click **Windows Credentials**
3. Look for entries starting with your service name

**Security:**
- Protected by Windows user account
- Accessible only when logged in as that user
- Encrypted using DPAPI

### Linux

**Viewing Secrets (GNOME):**
1. Open **Seahorse** (Passwords and Keys)
2. Look under **Login** keyring
3. Search for your service name

**Viewing Secrets (KDE):**
1. Open **KWalletManager**
2. Look for entries under your service name

**Headless Servers:**
```bash
# Start the keyring daemon
eval $(gnome-keyring-daemon --start --components=secrets)
export GNOME_KEYRING_CONTROL

# Or use dbus-run-session for scripts
dbus-run-session -- your-app
```

**Security:**
- Secrets encrypted with login password
- Keyring unlocked automatically on login (usually)
- May require manual unlock on headless systems

## Error Handling

```go
import (
    "errors"
    "github.com/plexusone/omnivault/vault"
)

secret, err := kr.Get(ctx, "my-secret")
if err != nil {
    switch {
    case errors.Is(err, vault.ErrSecretNotFound):
        // Secret doesn't exist
        log.Println("Secret not found, using default")

    case errors.Is(err, vault.ErrAccessDenied):
        // Permission denied (e.g., keychain locked)
        log.Println("Access denied - is the keychain unlocked?")

    case errors.Is(err, vault.ErrClosed):
        // Provider was closed
        log.Println("Provider is closed")

    default:
        // Other error (network, daemon not running, etc.)
        log.Printf("Error accessing keyring: %v", err)
    }
}
```

## Limitations

1. **No Native Enumeration**: OS keyrings don't support listing all entries. This provider maintains an internal index to enable `List()`, stored as a special keyring entry.

2. **Service Name Scope**: Secrets are namespaced by service name. Different service names create separate "buckets" of secrets.

3. **String Values Only**: OS keyrings store strings. Binary data should be base64-encoded.

4. **User Scope**: Secrets are tied to the current user account and cannot be shared across users.

5. **Size Limits**:
   - macOS: ~16KB per secret
   - Windows: ~2.5KB per credential
   - Linux: Varies by implementation

## Security Considerations

- **Don't log secrets**: Never log secret values, even in debug mode
- **Clear memory**: Go doesn't guarantee memory clearing, but avoid keeping secrets in memory longer than needed
- **Service name**: Use a unique service name to avoid conflicts with other applications
- **Access control**: On shared systems, be aware that other processes running as the same user can access the keyring

## Troubleshooting

### "secret not found" on Linux

Ensure the Secret Service daemon is running:
```bash
# Check if running
ps aux | grep gnome-keyring

# Start it
gnome-keyring-daemon --start --components=secrets
```

### Keychain prompts on every access (macOS)

Your application may need to be code-signed, or you need to grant "Always Allow" access in the Keychain Access prompt.

### "access denied" errors

- **macOS**: Keychain may be locked. Unlock via Keychain Access or `security unlock-keychain`
- **Linux**: Keyring may require manual unlock. Check Seahorse or KWalletManager
- **Windows**: Ensure you're running as the user who created the credentials

## Contributing

Contributions are welcome! Please submit pull requests or create issues for bugs and feature requests.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Related Projects

- [OmniVault](https://github.com/plexusone/omnivault) - Core library
- [zalando/go-keyring](https://github.com/zalando/go-keyring) - Underlying keyring library
- [99designs/keyring](https://github.com/99designs/keyring) - Alternative keyring library

 [build-status-svg]: https://github.com/plexusone/omnivault-keyring/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/plexusone/omnivault-keyring/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/plexusone/omnivault-keyring/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/plexusone/omnivault-keyring/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/plexusone/omnivault-keyring
 [goreport-url]: https://goreportcard.com/report/github.com/plexusone/omnivault-keyring
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/plexusone/omnivault-keyring
 [docs-godoc-url]: https://pkg.go.dev/github.com/plexusone/omnivault-keyring
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/plexusone/omnivault-keyring/blob/master/LICENSE
 [used-by-svg]: https://sourcegraph.com/github.com/plexusone/omnivault-keyring/-/badge.svg
 [used-by-url]: https://sourcegraph.com/github.com/plexusone/omnivault-keyring?badge