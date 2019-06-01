package wallet

import (
	"encoding/json"
	"log"
)

func (w *Wallet) Check() {
	ok, operation, err := w.storage.CheckWalletLock()

	if err != nil {
		panic(err)
	}

	if ok {
		return
	}

	var walletOperation map[string]interface{}

	err = json.Unmarshal([]byte(operation), &walletOperation)

	if err != nil {
		panic(err)
	}

	log.Fatalf(
		"FATAL: refusing to start processing because it was interrupted "+
			"during wallet operation and left in an inconsistent state. "+
			"Operation was %v with data %+v. Please check if it was completed "+
			"manually and update processing DB accordingly.\n"+
			"The following request can be executed in processing DB to let it "+
			"start again: "+
			"\"DELETE FROM metadata WHERE key = 'wallet_operation'\"",
		walletOperation["operation"], walletOperation["tx"],
	)
}
