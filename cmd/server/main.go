package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/go-fuego/fuego"

	"go-port/internal/db"
	"go-port/internal/flickr"
	"go-port/internal/handlers"
	"go-port/internal/workflows"
)

func main() {
	if err := run(); err != nil {
		// %+v prints the cockroachdb/errors stack trace.
		log.Fatalf("%+v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	adminPasswordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminPasswordHash == "" {
		return errors.New("ADMIN_PASSWORD_HASH is required; generate one with go run ./cmd/adminhash")
	}
	uploadDir := envOr("UPLOAD_DIR", "uploads")
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		return errors.Wrapf(err, "create upload directory %q", uploadDir)
	}

	// Application database: migrate with goose, query with sqlc.
	appDB, err := db.Connect(envOr("APP_DB", "app.db"))
	if err != nil {
		return err
	}
	defer appDB.Close()
	if err := db.Migrate(appDB); err != nil {
		return err
	}
	queries := db.New(appDB)
	var flickrCipher *flickr.Cipher
	if key := os.Getenv("FLICKR_TOKEN_ENCRYPTION_KEY"); key != "" {
		flickrCipher, err = flickr.NewCipher(key)
		if err != nil {
			return errors.Wrap(err, "configure Flickr token encryption")
		}
	}
	flickrService := &flickr.Service{
		Store: flickr.Store{DB: appDB, Cipher: flickrCipher},
		Client: flickr.NewClient(flickr.Config{
			APIKey: os.Getenv("FLICKR_API_KEY"), APISecret: os.Getenv("FLICKR_API_SECRET"),
			PublicBaseURL: os.Getenv("PUBLIC_BASE_URL"),
		}),
	}

	// DBOS keeps its system tables in a separate SQLite file so durable
	// workflow bookkeeping never contends with application writes.
	dbosDB, err := db.Connect(envOr("DBOS_DB", "dbos.db"))
	if err != nil {
		return err
	}
	defer dbosDB.Close()
	dbosCtx, err := dbos.NewDBOSContext(ctx, dbos.Config{
		AppName:        "go-port",
		SqliteSystemDB: dbosDB,
		Logger:         slog.Default(),
	})
	if err != nil {
		return errors.Wrap(err, "create DBOS context")
	}
	workflows.Register(dbosCtx, flickrService)
	if err := dbos.Launch(dbosCtx); err != nil {
		return errors.Wrap(err, "launch DBOS")
	}
	defer dbos.Shutdown(dbosCtx, 5*time.Second)

	s := fuego.NewServer(
		fuego.WithAddr(envOr("ADDR", "localhost:8080")),
	)
	handlers.Register(s, handlers.Resource{
		Queries:           queries,
		DBOS:              dbosCtx,
		AppDB:             appDB,
		DBOSDB:            dbosDB,
		AdminUsername:     envOr("ADMIN_USERNAME", "admin"),
		AdminPasswordHash: adminPasswordHash,
		UploadDir:         uploadDir,
		Flickr:            flickrService,
	})

	// Serve until the context is cancelled by SIGINT/SIGTERM, then drain
	// in-flight requests before the deferred DBOS/database shutdowns run.
	errCh := make(chan error, 1)
	go func() { errCh <- s.Run() }()
	select {
	case err := <-errCh:
		return errors.Wrap(err, "run server")
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return errors.Wrap(s.Shutdown(shutdownCtx), "shutdown server")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
