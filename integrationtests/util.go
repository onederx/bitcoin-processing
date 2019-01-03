package integrationtests

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"reflect"
	"testing"
	"time"
)

const waitForEventRetries = 120

func getFullSourcePath(dirName string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(path.Dir(cwd), dirName)
}

func waitForEventOrPanic(callback func() error) {
	err := waitForEvent(callback)
	if err != nil {
		panic(err)
	}
}

func waitForEvent(callback func() error) error {
	retries := waitForEventRetries

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		err := callback()
		if err != nil {
			retries--
			if retries <= 0 {
				return err
			}
		} else {
			return nil
		}
	}
	return nil
}

func waitForPort(host string, port uint16) {
	waitForEventOrPanic(func() error {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	})
}

func compareMetainfo(t *testing.T, got, want interface{}) {
	gotJSON, err := json.MarshalIndent(got, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal metainfo %#v to JSON for comparison: %s", got, err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal metainfo %#v to JSON for comparison: %s", want, err)
	}
	gotJSONStr, wantJSONStr := string(gotJSON), string(wantJSON)

	var gotUnified, wantUnified interface{}

	err = json.Unmarshal(gotJSON, &gotUnified)

	if err != nil {
		t.Fatalf("Failed to unmarshal metainfo %s back from JSON for comparison: %s", gotJSONStr, err)
	}

	err = json.Unmarshal(wantJSON, &wantUnified)

	if err != nil {
		t.Fatalf("Failed to unmarshal metainfo %s back from JSON for comparison: %s", wantJSONStr, err)
	}

	if !reflect.DeepEqual(gotUnified, wantUnified) {
		t.Fatalf("Unexpected metainfo. Got %s, wanted %s", gotJSONStr, wantJSONStr)
	}
}
