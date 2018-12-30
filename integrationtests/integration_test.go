// +build integration

package integrationtests

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestSmoke(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stop(ctx)
	env.waitForLoad()
	err = env.startProcessingWithDefaultSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.stopProcessing(ctx)
	env.waitForProcessing()

	resp, err := http.Get(fmt.Sprintf("http://%s:8000/get_events", env.processing.ip))

	if err != nil {
		t.Fatalf("Request to processing API get_events failed %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Request to processing API get_events returned non-200 status %d", resp.StatusCode)
	}
}
