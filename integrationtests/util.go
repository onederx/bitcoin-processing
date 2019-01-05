package integrationtests

import (
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"testing"
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

func waitForEventOrFailTest(t *testing.T, callback func() error) {
	err := waitForEvent(callback)
	if err != nil {
		t.Fatal(err)
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

func getNewAddressForWithdrawOrFail(t *testing.T, env *testEnvironment) string {
	addressDecoded, err := env.regtest["node-client"].nodeAPI.CreateNewAddress()

	if err != nil {
		t.Fatalf("Failed to request new address from client node: %v", err)
	}
	return addressDecoded.EncodeAddress()
}
