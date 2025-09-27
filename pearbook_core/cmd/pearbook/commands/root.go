package commands

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pearbook",
	Short: "PearBook - Distributed Expense Tracker",
	Long: `PearBook is a decentralized application that allows users to track shared expenses
without relying on central servers. It uses Conflict-Free Replicated Data Types (CRDTs)
over a Kademlia DHT to ensure eventual consistency across distributed nodes.

This CLI provides commands to run the server, generate cryptographic keys,
and sign operation data for secure API interactions.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pearbook.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
