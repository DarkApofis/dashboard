package postgres_test

// Test bootstrap for the PostgreSQL integration tests.
//
// The goal is that `go test ./...` runs everything, with no setup step to forget.
// Two ways to get a database, in this order:
//
//  1. TEST_DATABASE_URL, when it is already set. Useful against the docker-compose
//     database, and it makes the test run fast because nothing is started.
//  2. Otherwise a throwaway PostgreSQL started with testcontainers-go, migrated from
//     ./migrations and removed when the package finishes.
//
// If neither is available (no Docker on the machine), the integration tests skip with
// an explanation instead of failing: a contributor without Docker still gets a green
// unit suite, and CI, which does have Docker, runs the whole thing.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	postgresImage   = "postgres:17-alpine"
	migrationsDir   = "../../../migrations"
	containerDBName = "payment_links_test"
	containerUser   = "payment_links"
	containerPass   = "payment_links"
)

var (
	// databaseURL is the connection string every integration test uses.
	databaseURL string
	// setupErr explains why databaseURL is empty, so the skip message is useful.
	setupErr error
)

func TestMain(m *testing.M) {
	code := func() int {
		// Cleanup has to run before os.Exit, which does not honour defers.
		terminate := setupDatabase()
		defer terminate()
		return m.Run()
	}()
	os.Exit(code)
}

// setupDatabase resolves a database to test against and returns its cleanup.
func setupDatabase() func() {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		databaseURL = url
		return func() {}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase(containerDBName),
		tcpostgres.WithUsername(containerUser),
		tcpostgres.WithPassword(containerPass),
		// Postgres logs "ready to accept connections" once while initialising the
		// data directory and again when it is genuinely ready; waiting for the
		// second occurrence is what avoids a flaky first connection.
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		setupErr = fmt.Errorf("could not start a PostgreSQL container: %w", err)
		return func() {}
	}

	terminate := func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		_ = testcontainers.TerminateContainer(container, testcontainers.StopContext(stopCtx))
	}

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		setupErr = fmt.Errorf("could not read the container connection string: %w", err)
		terminate()
		return func() {}
	}

	if err := migrate(ctx, url); err != nil {
		setupErr = fmt.Errorf("could not migrate the test database: %w", err)
		terminate()
		return func() {}
	}

	databaseURL = url
	return terminate
}

// migrate applies every *.up.sql in order. The migrate binary is deliberately not
// required: the tests run the same SQL the real migrations ship, so a migration that
// does not apply cleanly fails the suite.
func migrate(ctx context.Context, url string) error {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found in %s", migrationsDir)
	}
	sort.Strings(files)

	for _, file := range files {
		statements, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		// No parameters, so pgx sends this with the simple protocol and a file with
		// several statements applies as one unit.
		if _, err := pool.Exec(ctx, string(statements)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

// requireDatabase returns the connection string, or skips the test explaining why
// there is none.
func requireDatabase(t *testing.T) string {
	t.Helper()

	if databaseURL == "" {
		t.Skipf("no test database available (%v); set TEST_DATABASE_URL or start Docker", setupErr)
	}
	return databaseURL
}
