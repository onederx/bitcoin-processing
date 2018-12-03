package main

import (
	"bytes"
	"encoding/json"
	"github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"log"
	"net/http"
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
				txIDJSON, err := json.Marshal(txID)
				if err != nil {
					log.Fatal(err)
				}
				resp, err := http.Post(
					urljoin(apiURL, "/"+command),
					"application/json",
					bytes.NewBuffer(txIDJSON),
				)
				if err != nil {
					log.Fatal(err)
				}
				defer resp.Body.Close()
				showResponse(resp.Body)
			},
		}
	}
	for command, description := range commands {
		cli.AddCommand(makeCancelOrConfirmCmd(command, description))
	}
}
