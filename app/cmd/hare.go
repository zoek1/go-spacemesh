package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)


// VersionCmd returns the current version of spacemesh
var HareCmd = &cobra.Command{
	Use:   "hare",
	Short: "start hare",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting hare")
	},
}

func init() {
	RootCmd.AddCommand(HareCmd)
}