package commands

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/khelechy/pearbook/pearbook_core/models"
	"github.com/khelechy/pearbook/pearbook_core/utils"

	"github.com/spf13/cobra"
)

// signCmd represents the sign command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign operation data with your private key",
	Long: `Sign operation data using your ECDSA private key.
This creates a signed operation that can be submitted to the PearBook server.
Supported operations: create_group, join_group, add_expense, approve_join

You can provide operation data either inline with --data or from a file with --data-file.`,
	Run: func(cmd *cobra.Command, args []string) {
		runSign(cmd)
	},
}

func init() {
	rootCmd.AddCommand(signCmd)

	// Add flags
	signCmd.Flags().StringP("key", "k", "private_key.pem", "Path to your private key file")
	signCmd.Flags().StringP("operation", "o", "", "Operation type (create_group, join_group, add_expense, approve_join)")
	signCmd.Flags().StringP("group-id", "g", "", "Group ID for the operation")
	signCmd.Flags().StringP("user-id", "u", "", "Your user ID")
	signCmd.Flags().StringP("data", "d", "", "JSON data for the operation")
	signCmd.Flags().StringP("data-file", "f", "", "JSON file containing operation data")
	signCmd.Flags().StringP("output", "O", "", "Output file for signed operation (default: stdout)")

	// Mark required flags
	signCmd.MarkFlagRequired("operation")
	signCmd.MarkFlagRequired("user-id")
	// Note: either --data or --data-file must be provided (validated in runSign)
}

func runSign(cmd *cobra.Command) {
	keyFile, _ := cmd.Flags().GetString("key")
	operation, _ := cmd.Flags().GetString("operation")
	groupID, _ := cmd.Flags().GetString("group-id")
	userID, _ := cmd.Flags().GetString("user-id")
	dataStr, _ := cmd.Flags().GetString("data")
	dataFile, _ := cmd.Flags().GetString("data-file")
	outputFile, _ := cmd.Flags().GetString("output")

	// Validate that either data or data-file is provided
	if dataStr == "" && dataFile == "" {
		fmt.Println("Error: either --data or --data-file must be specified")
		os.Exit(1)
	}
	if dataStr != "" && dataFile != "" {
		fmt.Println("Error: cannot specify both --data and --data-file")
		os.Exit(1)
	}

	// Load private key
	privateKey, err := loadPrivateKey(keyFile)
	if err != nil {
		fmt.Printf("Failed to load private key: %v\n", err)
		os.Exit(1)
	}

	// Get data string from file if specified
	if dataFile != "" {
		dataBytes, err := os.ReadFile(dataFile)
		if err != nil {
			fmt.Printf("Failed to read data file: %v\n", err)
			os.Exit(1)
		}
		dataStr = string(dataBytes)
	}

	// Parse operation data
	var data map[string]interface{}
	err = json.Unmarshal([]byte(dataStr), &data)
	if err != nil {
		fmt.Printf("Failed to parse operation data JSON: %v\n", err)
		os.Exit(1)
	}

	// Create signed operation
	signedOp := models.SignedOperation{
		Operation: operation,
		GroupID:   groupID,
		UserID:    userID,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	// Create operation data for signing
	opData := utils.CreateOperationData(signedOp.Operation, signedOp.GroupID, signedOp.UserID, signedOp.Timestamp, signedOp.Data)

	// Sign the operation
	signature, err := utils.SignData(privateKey, opData)
	if err != nil {
		fmt.Printf("Failed to sign operation: %v\n", err)
		os.Exit(1)
	}

	signedOp.Signature = signature

	// Output the signed operation
	output, err := json.MarshalIndent(signedOp, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal signed operation: %v\n", err)
		os.Exit(1)
	}

	if outputFile != "" {
		err = os.WriteFile(outputFile, output, 0644)
		if err != nil {
			fmt.Printf("Failed to write output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Signed operation saved to: %s\n", outputFile)
	} else {
		fmt.Println(string(output))
	}
}

func loadPrivateKey(filename string) (*ecdsa.PrivateKey, error) {
	// Read the key file
	keyData, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Decode PEM
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("invalid key type: %s", block.Type)
	}

	// Parse the private key
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}
