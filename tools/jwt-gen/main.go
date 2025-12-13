package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/moroshma/MiniToolStreamConnector/auth"
)

var (
	clientID        = flag.String("client", "", "Client ID (required)")
	subjects        = flag.String("subjects", "*", "Comma-separated list of allowed subjects (e.g., 'images.*,logs.*')")
	permissions     = flag.String("permissions", "publish,subscribe,fetch", "Comma-separated list of permissions")
	duration        = flag.Duration("duration", 24*time.Hour, "Token validity duration")
	vaultAddr       = flag.String("vault-addr", os.Getenv("VAULT_ADDR"), "Vault address")
	vaultToken      = flag.String("vault-token", os.Getenv("VAULT_TOKEN"), "Vault token")
	vaultPath       = flag.String("vault-path", "secret/data/minitoolstream/jwt", "Vault path for JWT keys")
	issuer          = flag.String("issuer", "minitoolstream", "JWT issuer")
	generateKeysCmd = flag.Bool("generate-keys", false, "Generate new RSA keys and save to Vault")
	showPublicKey   = flag.Bool("show-public-key", false, "Show public key from Vault")
)

func main() {
	flag.Parse()

	if *vaultAddr == "" {
		log.Fatal("Vault address is required (use -vault-addr or VAULT_ADDR env var)")
	}

	if *vaultToken == "" {
		log.Fatal("Vault token is required (use -vault-token or VAULT_TOKEN env var)")
	}

	// Create Vault client
	config := vault.DefaultConfig()
	config.Address = *vaultAddr

	vaultClient, err := vault.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create Vault client: %v", err)
	}
	vaultClient.SetToken(*vaultToken)

	ctx := context.Background()

	// Handle generate-keys command
	if *generateKeysCmd {
		if err := generateAndSaveKeys(ctx, vaultClient); err != nil {
			log.Fatalf("Failed to generate keys: %v", err)
		}
		fmt.Println("✓ RSA keys generated and saved to Vault successfully")
		return
	}

	// Handle show-public-key command
	if *showPublicKey {
		if err := showPublicKeyFromVault(ctx, vaultClient); err != nil {
			log.Fatalf("Failed to show public key: %v", err)
		}
		return
	}

	// Generate JWT token
	if *clientID == "" {
		flag.Usage()
		log.Fatal("\nClient ID is required for token generation")
	}

	if err := generateToken(ctx, vaultClient); err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}
}

func generateAndSaveKeys(ctx context.Context, vaultClient *vault.Client) error {
	jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, *vaultPath, *issuer)
	if err != nil {
		return err
	}

	return jwtManager.SaveKeysToVault(ctx, vaultClient)
}

func showPublicKeyFromVault(ctx context.Context, vaultClient *vault.Client) error {
	jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, *vaultPath, *issuer)
	if err != nil {
		return err
	}

	publicKey, err := jwtManager.GetPublicKey()
	if err != nil {
		return err
	}

	fmt.Println("Public Key (PEM format):")
	fmt.Println(publicKey)
	return nil
}

func generateToken(ctx context.Context, vaultClient *vault.Client) error {
	// Create JWT manager from Vault
	jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, *vaultPath, *issuer)
	if err != nil {
		return err
	}

	// Parse subjects
	allowedSubjects := []string{}
	if *subjects != "" {
		allowedSubjects = strings.Split(*subjects, ",")
		for i := range allowedSubjects {
			allowedSubjects[i] = strings.TrimSpace(allowedSubjects[i])
		}
	}

	// Parse permissions
	perms := []string{}
	if *permissions != "" {
		perms = strings.Split(*permissions, ",")
		for i := range perms {
			perms[i] = strings.TrimSpace(perms[i])
		}
	}

	// Generate token
	token, err := jwtManager.GenerateToken(*clientID, allowedSubjects, perms, *duration)
	if err != nil {
		return err
	}

	// Print token info
	fmt.Printf("JWT Token generated successfully:\n\n")
	fmt.Printf("Client ID:        %s\n", *clientID)
	fmt.Printf("Allowed Subjects: %v\n", allowedSubjects)
	fmt.Printf("Permissions:      %v\n", perms)
	fmt.Printf("Valid For:        %v\n", *duration)
	fmt.Printf("Issuer:           %s\n\n", *issuer)
	fmt.Printf("Token:\n%s\n\n", token)
	fmt.Printf("Use this token in your client by setting the Authorization header:\n")
	fmt.Printf("Authorization: Bearer %s\n", token)

	return nil
}
