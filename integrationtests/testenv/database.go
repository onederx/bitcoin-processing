package testenv

import (
	"context"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

const (
	dbImageName     = "postgres:10.5"
	dbContainerName = "bitcoin-processing-integration-test-db"
)

func (e *TestEnvironment) StartDatabase(ctx context.Context) error {
	log.Printf("Starting postgres")

	containerConfig := &container.Config{Image: dbImageName}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(e.network),
		Binds: []string{
			getFullSourcePath("tools/create-user-and-db.sql") + ":/create-user-and-db.sql",
			getFullSourcePath("tools/init-db.sql") + ":/init-db.sql",
			getFullSourcePath("integrationtests/testdata/docker-create-user-and-db-and-initialize.sh") +
				":/docker-entrypoint-initdb.d/initdb.sh",
		},
	}

	resp, err := e.cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, dbContainerName)
	if err != nil {
		return err
	}
	e.DB = &containerInfo{
		name: "db",
		ID:   resp.ID,
	}

	return e.LaunchDatabaseContainer(ctx)
}

func (e *TestEnvironment) stopDatabase(ctx context.Context) error {
	log.Printf("trying to stop db container")
	if e.DB == nil {
		log.Printf("seems that db is not running")
		return nil
	}

	if err := e.cli.ContainerStop(ctx, e.DB.ID, nil); err != nil {
		return err
	}

	log.Printf("db container stopped: id=%v", e.DB.ID)

	err := e.cli.ContainerRemove(ctx, e.DB.ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})

	if err != nil {
		return err
	}

	log.Printf("db container removed: id=%v", e.DB.ID)

	e.DB = nil
	return nil
}

func (e *TestEnvironment) WaitForDatabase() {
	log.Printf("waiting for postgres to start")
	waitForPort(e.DB.IP, 5432)
	log.Printf("postgres started")
}

func (e *TestEnvironment) KillDatabase(ctx context.Context, removeInfo bool) error {
	db := e.DB
	if removeInfo {
		e.DB = nil
	}
	return e.cli.ContainerKill(ctx, db.ID, "SIGKILL")
}

func (e *TestEnvironment) LaunchDatabaseContainer(ctx context.Context) error {
	err := e.cli.ContainerStart(ctx, e.DB.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	e.DB.IP = e.getContainerIP(ctx, e.DB.ID)

	log.Printf("db container started: id=%v", e.DB.ID)

	return e.writeContainerLogs(ctx, e.DB, "postgres.log")
}
