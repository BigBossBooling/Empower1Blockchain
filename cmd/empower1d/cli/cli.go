package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/empower1/empower1/internal/core"
)

func NewCLI(bc *core.Blockchain) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "empower1d",
		Short: "EmPower1 is a humanitarian blockchain.",
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing
		},
	}

	var addBlockCmd = &cobra.Command{
		Use:   "addblock [data]",
		Short: "Add a block to the blockchain",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bc.AddBlock([]*core.Transaction{})
			fmt.Println("Success!")
		},
	}

	var printChainCmd = &cobra.Command{
		Use:   "printchain",
		Short: "Print all the blocks of the blockchain",
		Run: func(cmd *cobra.Command, args []string) {
			bci := bc.Iterator()

			for {
				block := bci.Next()

				fmt.Printf("Prev. hash: %x\n", block.PrevHash)
				fmt.Printf("Hash: %x\n", block.Hash)
				fmt.Println()

				if len(block.PrevHash) == 0 {
					break
				}
			}
		},
	}

	rootCmd.AddCommand(addBlockCmd)
	rootCmd.AddCommand(printChainCmd)

	return rootCmd
}
