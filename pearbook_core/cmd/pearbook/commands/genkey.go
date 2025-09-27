package commands

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/khelechy/pearbook/pearbook_core/utils"

	"github.com/spf13/cobra"
)

// genkeyCmd represents the genkey command
var genkeyCmd = &cobra.Command{
	Use:   "genkey",
	Short: "Generate ECDSA key pair for cryptographic operations",
	Long: `Generate a new ECDSA key pair using the P-256 curve.
The private key will be saved to a PEM file and the public key will be displayed in hex format.
Use this key pair for signing operations and sharing your public key with groups.`,
	Run: func(cmd *cobra.Command, args []string) {
		runGenKey(cmd)
	},
}

func init() {
	rootCmd.AddCommand(genkeyCmd)

	// Add flags for output file
	genkeyCmd.Flags().StringP("output", "o", "private_key.pem", "Output file for the private key")
	genkeyCmd.Flags().BoolP("force", "f", false, "Overwrite existing key file")
}

func runGenKey(cmd *cobra.Command) {
	outputFile, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")

	// Check if file exists
	if _, err := os.Stat(outputFile); err == nil && !force {
		fmt.Printf("Key file '%s' already exists. Use --force to overwrite.\n", outputFile)
		os.Exit(1)
	}

	// Generate ECDSA key pair using utils function
	privateKey, publicKeyBytes, err := utils.GenerateKeyPair()
	if err != nil {
		fmt.Printf("Failed to generate key pair: %v\n", err)
		os.Exit(1)
	}

	// Encode private key to PEM
	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		fmt.Printf("Failed to marshal private key: %v\n", err)
		os.Exit(1)
	}

	privateKeyPEM := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	// Write private key to file
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Failed to create key file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	err = pem.Encode(file, privateKeyPEM)
	if err != nil {
		fmt.Printf("Failed to write private key: %v\n", err)
		os.Exit(1)
	}

	// Generate user ID from public key
	userId := utils.GenerateUserID(publicKeyBytes)

	// Display public key in hex format for easy copying
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	fmt.Printf("✓ ECDSA key pair generated successfully!\n\n")
	fmt.Printf("Private key saved to: %s\n", outputFile)
	fmt.Printf("Public key (hex format - use this for create_group/join_group operations):\n\n")
	fmt.Printf("%s\n\n", publicKeyHex)
	fmt.Printf("User ID for this key: %s\n\n", userId)

	fmt.Printf("Keep your private key secure and never share it!\n")
	fmt.Printf("Use the public key when joining groups or for signature verification.\n")
}
