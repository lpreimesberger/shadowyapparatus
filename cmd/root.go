package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "shadowy",
	Short: "Shadowy - A proof-of-storage cryptocurrency",
	Long: `Shadowy is a proof-of-storage cryptocurrency implementation.
It allows you to plot storage space and participate in the network.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(plotCmd)
}