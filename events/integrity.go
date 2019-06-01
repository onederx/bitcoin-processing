package events

import (
	"encoding/json"
	"log"
)

func (w *eventBroker) Check() {
	ok, operation, err := w.storage.CheckHTTPCallbackLock()

	if err != nil {
		panic(err)
	}

	if ok {
		return
	}

	var seq int

	err = json.Unmarshal([]byte(operation), &seq)

	if err != nil {
		panic(err)
	}

	log.Fatalf(
		"FATAL: refusing to start processing because it was interrupted "+
			"during sending event with seq %d via HTTP callback. Please check "+
			"if event was delivered and update last_http_sent_seq in "+
			"\"metadata\" table accordingly\n"+
			"The following request can be executed in processing DB to let it "+
			"start again: \"DELETE FROM metadata WHERE key = "+
			"'http_callback_operation'\"", seq)
}
