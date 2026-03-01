// Example usage of omnivault-keyring
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/plexusone/omnivault"
	"github.com/plexusone/omnivault/vault"

	keyring "github.com/plexusone/omnivault-keyring"
)

func main() {
	ctx := context.Background()

	// Create keyring provider
	kr := keyring.New(keyring.Config{
		ServiceName: "myapp",
		JSONFormat:  true, // Enable multi-field secrets
	})

	fmt.Printf("Using backend: %s\n", kr.Backend())
	fmt.Printf("Service name: %s\n\n", kr.ServiceName())

	// Example 1: Direct usage
	fmt.Println("=== Direct Usage ===")

	// Store a secret
	err := kr.Set(ctx, "database/credentials", &vault.Secret{
		Value: "mypassword",
		Fields: map[string]string{
			"username": "admin",
			"host":     "localhost",
			"port":     "5432",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Stored database credentials")

	// Retrieve the secret
	secret, err := kr.Get(ctx, "database/credentials")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Password: %s\n", secret.Value)
	fmt.Printf("Username: %s\n", secret.Fields["username"])
	fmt.Printf("Host: %s\n\n", secret.Fields["host"])

	// Example 2: With OmniVault client
	fmt.Println("=== With OmniVault Client ===")

	client, err := omnivault.NewClient(omnivault.Config{
		CustomVault: kr,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Use client methods
	err = client.SetValue(ctx, "api/token", "secret-token-12345")
	if err != nil {
		log.Fatal(err)
	}

	token, err := client.GetValue(ctx, "api/token")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("API Token: %s\n\n", token)

	// Example 3: With resolver
	fmt.Println("=== With Resolver ===")

	resolver := omnivault.NewResolver()
	resolver.Register("keyring", kr)

	value, err := resolver.Resolve(ctx, "keyring://api/token")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Resolved via URI: %s\n\n", value)

	// List all secrets
	fmt.Println("=== List Secrets ===")
	paths, err := kr.List(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
	for _, path := range paths {
		fmt.Printf("  - %s\n", path)
	}

	// Cleanup
	fmt.Println("\n=== Cleanup ===")
	_ = kr.Delete(ctx, "database/credentials")
	_ = kr.Delete(ctx, "api/token")
	fmt.Println("Deleted test secrets")

	client.Close()
}
