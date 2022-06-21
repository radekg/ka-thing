package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/radekg/ka-thing/cmd/start"
)

var rootCmd = &cobra.Command{
	Use:   "ka-thing",
	Short: "ka-thing",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(start.Command)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
