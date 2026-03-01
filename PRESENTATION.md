---
marp: true
theme: plexusone
paginate: true
---

# OmniVault Keyring

## Cross-Platform OS Credential Store for Go

Secure secrets using native OS keyrings

---

# The Problem

- Applications need to store sensitive credentials securely
- Environment variables are not secure (visible in process lists)
- Custom encryption requires key management complexity
- Different platforms have different secure storage mechanisms

---

# The Solution

**OmniVault Keyring** leverages your OS's built-in credential store:

| Platform | Backend |
|----------|---------|
| macOS | Keychain |
| Windows | Credential Manager |
| Linux | Secret Service (GNOME Keyring / KWallet) |

*No custom encryption needed — use what your OS already provides*

---

# Key Features

- **Cross-Platform** — Single API for all major operating systems
- **Native Security** — Uses OS-provided secure storage
- **Multi-Field Secrets** — Store complex credentials (username, password, host, etc.)
- **OmniVault Compatible** — Implements the `vault.Vault` interface
- **URI Resolution** — Use `keyring://` URIs with OmniVault's resolver
- **Listing Support** — Enumerate stored secrets via internal index

---

# Quick Start

```go
import (
    "github.com/plexusone/omnivault-keyring"
    "github.com/plexusone/omnivault/vault"
)

// Create a keyring provider
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
})

// Store a secret
kr.Set(ctx, "api-key", &vault.Secret{Value: "sk-12345"})

// Retrieve the secret
secret, _ := kr.Get(ctx, "api-key")
fmt.Println(secret.Value) // sk-12345
```

---

# Multi-Field Secrets

Store complex credentials with JSON format enabled:

```go
kr := keyring.New(keyring.Config{
    ServiceName: "myapp",
    JSONFormat:  true,
})

kr.Set(ctx, "database/prod", &vault.Secret{
    Value: "super-secret-password",
    Fields: map[string]string{
        "username": "db_admin",
        "host":     "db.example.com",
        "port":     "5432",
    },
})

secret, _ := kr.Get(ctx, "database/prod")
fmt.Printf("Host: %s, User: %s", secret.Fields["host"], secret.Fields["username"])
```

---

# OmniVault Integration

Use as a custom vault backend:

```go
import "github.com/plexusone/omnivault"

client, _ := omnivault.NewClient(omnivault.Config{
    CustomVault: keyring.New(keyring.Config{
        ServiceName: "myapp",
    }),
})

client.SetValue(ctx, "api/token", "secret-token")
token, _ := client.GetValue(ctx, "api/token")
```

---

# URI Resolution

Combine with other providers using the resolver:

```go
resolver := omnivault.NewResolver()
resolver.Register("keyring", keyring.NewWithServiceName("myapp"))
resolver.Register("env", env.New())

// Resolve from different sources
dbPassword, _ := resolver.Resolve(ctx, "keyring://database/password")
apiKey, _ := resolver.Resolve(ctx, "env://API_KEY")

// Access specific fields
refreshToken, _ := resolver.Resolve(ctx, "keyring://oauth/github#refresh_token")
```

---

# Listing Secrets

Enumerate stored secrets by prefix:

```go
// Store some secrets
kr.Set(ctx, "database/prod", &vault.Secret{Value: "pass1"})
kr.Set(ctx, "database/staging", &vault.Secret{Value: "pass2"})
kr.Set(ctx, "api/github", &vault.Secret{Value: "token1"})

// List all
all, _ := kr.List(ctx, "")
// ["database/prod", "database/staging", "api/github"]

// List by prefix
dbSecrets, _ := kr.List(ctx, "database/")
// ["database/prod", "database/staging"]
```

---

# Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Your Application                     │
├─────────────────────────────────────────────────────────┤
│                  omnivault-keyring                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │ vault.Vault │  │ JSON Format │  │ Index Tracking  │  │
│  │  Interface  │  │   Support   │  │   for List()    │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────┤
│                  zalando/go-keyring                     │
├───────────────┬─────────────────┬───────────────────────┤
│ macOS Keychain│ Windows CredMgr │ Linux Secret Service  │
└───────────────┴─────────────────┴───────────────────────┘
```

---

# Configuration

```go
type Config struct {
    // ServiceName namespaces your secrets
    // Default: "omnivault"
    ServiceName string

    // JSONFormat enables multi-field secrets
    // Default: false
    JSONFormat bool

    // OnIndexError callback for index operation errors
    // Default: nil (errors silently ignored)
    OnIndexError func(op string, err error)
}
```

---

# Error Handling

```go
secret, err := kr.Get(ctx, "my-secret")
if err != nil {
    switch {
    case errors.Is(err, vault.ErrSecretNotFound):
        // Secret doesn't exist
    case errors.Is(err, vault.ErrAccessDenied):
        // Keychain locked or permission denied
    case errors.Is(err, vault.ErrClosed):
        // Provider was closed
    default:
        // Other error
    }
}
```

---

# Security Considerations

- **Native encryption** — Secrets encrypted by OS, not custom code
- **User-scoped** — Secrets tied to current user account
- **No logging** — Never log secret values
- **Unique namespacing** — Use distinct service names per application
- **Memory awareness** — Don't keep secrets in memory longer than needed

---

# Platform Notes

**macOS**
- First access prompts for keychain password
- Unsigned apps trigger additional security prompts

**Windows**
- Protected by Windows user account via DPAPI

**Linux**
- Requires Secret Service daemon (gnome-keyring or kwallet)
- May need manual unlock on headless systems

---

# Size Limits

| Platform | Max Secret Size |
|----------|-----------------|
| macOS | ~16 KB |
| Windows | ~2.5 KB |
| Linux | Varies by implementation |

*For larger data, store a reference and keep data elsewhere*

---

<!-- _class: lead -->

# Get Started

```bash
go get github.com/plexusone/omnivault-keyring
```

**Documentation**
github.com/plexusone/omnivault-keyring

**Related Projects**
- [OmniVault](https://github.com/plexusone/omnivault) — Core library
- [zalando/go-keyring](https://github.com/zalando/go-keyring) — Underlying keyring library
