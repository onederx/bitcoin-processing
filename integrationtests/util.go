package integrationtests

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/onederx/bitcoin-processing/api"
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

func getGoodResponseResultOrFail(t *testing.T, resp *http.Response, err error) interface{} {
	url := resp.Request.URL.String()

	if err != nil {
		t.Fatalf("Request to processing API %s failed %s", url, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Request to processing API %s returned non-200 status %d", url, resp.StatusCode)
	}

	var responseData api.HttpAPIResponse

	err = json.NewDecoder(resp.Body).Decode(&responseData)
	if err != nil {
		t.Fatalf("Failed to JSON-decode API response from %s %s", url, err)
	}
	if responseData.Error != "ok" {
		t.Fatalf("Unexpected error from %s API %s", url, responseData.Error)
	}
	return responseData.Result
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
