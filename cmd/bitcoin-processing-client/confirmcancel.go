package main

import (
	"log"

	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"

	"github.com/onederx/bitcoin-processing/api/client"
)

func init() {
	commands := map[string]string{
		"confirm":        "Confirm transaction pending manual confirmation",
		"cancel_pending": "Cancel pending transaction",
	}
	makeCancelOrConfirmCmd := func(command, description string) *cobra.Command {
		return &cobra.Command{
			Use:     command + " TX_ID",
			Example: command + " aec79cbf-79c4-46ef-a54f-63a0cf451fe2",
			Short:   description,
			Args:    cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				txID, err := uuid.FromString(args[0])
				if err != nil {
					log.Fatal(err)
				}
				cli := client.NewClient(apiURL)
				switch command {
				case "confirm":
					err = cli.Confirm(txID)
				case "cancel_pending":
					err = cli.Cancel(txID)
				default:
					panic("Unexpected command " + command)
				}
				if err != nil {
					log.Fatal(err)
				}
				log.Println("OK")
			},
		}
	}
	for command, description := range commands {
		cli.AddCommand(makeCancelOrConfirmCmd(command, description))
	}
}
