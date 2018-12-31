package integrationtests

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

func (e *testEnvironment) startDatabase(ctx context.Context) error {
	log.Printf("Starting postgres")

	containerConfig := &container.Config{Image: dbImageName}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(e.network),
		AutoRemove:  true,
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
	e.db = &containerInfo{
		name: "db",
		id:   resp.ID,
	}

	err = e.cli.ContainerStart(ctx, e.db.id, types.ContainerStartOptions{})
	if err != nil {
		return err
	}
	e.db.ip = e.getContainerIP(ctx, resp.ID)

	log.Printf("db container started: id=%v", e.db.id)
	return nil
}

func (e *testEnvironment) stopDatabase(ctx context.Context) error {
	log.Printf("trying to stop db container")
	if e.db == nil {
		log.Printf("seems that db is not running")
		return nil
	}

	if err := e.cli.ContainerStop(ctx, e.db.id, nil); err != nil {
		return err
	}

	log.Printf("db container stopped: id=%v", e.db.id)
	e.db = nil
	return nil
}

func (e *testEnvironment) waitForDatabase() {
	log.Printf("waiting for postgres to start")
	waitForPort(e.db.ip, 5432)
	log.Printf("postgres started")
}
