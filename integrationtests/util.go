package integrationtests

import (
	"fmt"
	"net"
	"os"
	"path"
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
