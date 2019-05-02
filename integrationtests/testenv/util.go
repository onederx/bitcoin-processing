package testenv

import (
	"fmt"
	"net"
	"os"
	"path"

	"github.com/onederx/bitcoin-processing/integrationtests/util"
)

func getFullSourcePath(dirName string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(path.Dir(cwd), dirName)
}

func waitForEventOrPanic(callback func() error) {
	err := util.WaitForEvent(callback)
	if err != nil {
		panic(err)
	}
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
