package client

import (
	"encoding/json"
	"testing"
)

func TestCallbackedHTTPAPIResponse(t *testing.T) {
	const testData = `{
		"error": "ok",
		"result": "EXPECTEDTESTRESULT"
	}`
	var (
		apiResponse       callbackedHTTPAPIResponse
		wantData          string
		callbackWasCalled bool
	)

	apiResponse.Result.unmarshalCb = func(result []byte) error {
		callbackWasCalled = true
		if err := json.Unmarshal(result, &wantData); err != nil {
			return err
		}
		return nil
	}

	err := json.Unmarshal([]byte(testData), &apiResponse)

	if err != nil {
		t.Fatal(err)
	}
	if !callbackWasCalled {
		t.Fatal("Unmarshal callback was not called")
	}
	if got, want := wantData, "EXPECTEDTESTRESULT"; got != want {
		t.Fatalf("Unmarshal callback did not produce expected data %s, "+
			"instead got %s", want, got)
	}
}
