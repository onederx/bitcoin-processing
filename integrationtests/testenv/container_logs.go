package testenv

import (
	"context"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

func (e *TestEnvironment) writeContainerLogs(ctx context.Context, container *containerInfo, filename string) error {
	logFile, err := os.OpenFile(
		filename,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)

	if err != nil {
		return err
	}

	logReader, err := e.cli.ContainerLogs(ctx, container.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     true,
	})

	if err != nil {
		return err
	}

	go func() {
		defer logReader.Close()
		defer logFile.Close()

		_, err := stdcopy.StdCopy(logFile, logFile, logReader)

		if err != nil {
			log.Printf("Log reader for container %s (id %s) got error %v",
				container.name, container.ID, err)
		} else {
			log.Printf("Done streaming logs from container %s (id %s)",
				container.name, container.ID)
		}
	}()
	log.Printf("Started streaming logs from container %s (id %s) to log %s",
		container.name, container.ID, filename,
	)
	return nil
}
