package integrationtests

import (
	"fmt"
	"net"
	"os"
	"path"
	"time"
)

func getFullSourcePath(dirName string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(path.Dir(cwd), dirName)
}

func waitForPort(host string, port uint16) {
	var err error
	retries := 120

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			retries--
			if retries > 0 {
				continue
			}
		}
		conn.Close()
		return
	}
	panic(err)
}
