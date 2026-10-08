package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
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
	publicAddr := envOr("ADDR", "127.0.0.1:8081")
	adminAddr := envOr("ADMIN_ADDR", "127.0.0.1:8082")
	development := os.Getenv("APP_ENV") == "development"
	publicURL, err := handlers.ValidateOrigin(os.Getenv("PUBLIC_BASE_URL"), publicAddr, development)
	if err != nil {
		return errors.Wrap(err, "public origin")
	}
	adminURL, err := handlers.ValidateOrigin(os.Getenv("ADMIN_BASE_URL"), adminAddr, development)
	if err != nil {
		return errors.Wrap(err, "admin origin")
	}
	publicListener, err := net.Listen("tcp", publicAddr)
	if err != nil {
		return err
	}
	defer publicListener.Close()
	adminListener, err := net.Listen("tcp", adminAddr)
	if err != nil {
		return err
	}
	defer adminListener.Close()
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
			PublicBaseURL: adminURL.String(),
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

	publicDB, err := db.ConnectReadOnly(envOr("APP_DB", "app.db"))
	if err != nil {
		return err
	}
	defer publicDB.Close()
	resource := handlers.Resource{
		Queries: queries, DBOS: dbosCtx, AppDB: appDB, DBOSDB: dbosDB,
		AdminUsername: envOr("ADMIN_USERNAME", "admin"), AdminPasswordHash: adminPasswordHash,
		UploadDir: uploadDir, Flickr: flickrService, PublicBaseURL: publicURL.String(), AdminBaseURL: adminURL.String(),
	}
	newServer := func() *fuego.Server {
		return fuego.NewServer(fuego.WithErrorSerializer(handlers.SafeError), fuego.WithEngineOptions(fuego.WithErrorHandler(func(_ context.Context, err error) error { return err })))
	}
	private := newServer()
	handlers.RegisterAdmin(private, resource)
	public := newServer()
	publicResource := resource
	publicResource.Queries = db.New(publicDB)
	publicResource.AppDB = publicDB
	publicResource.Flickr = &flickr.Service{Store: flickr.Store{DB: publicDB}}
	handlers.RegisterPublic(public, publicResource)
	servers := []*http.Server{
		{Handler: handlers.Boundary(public.Mux, publicURL, publicURL.String(), false), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10},
		{Handler: handlers.Boundary(private.Mux, adminURL, publicURL.String(), true), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10},
	}
	errCh := make(chan error, 2)
	go func() { errCh <- servers[0].Serve(publicListener) }()
	go func() { errCh <- servers[1].Serve(adminListener) }()
	maintenanceCtx, maintenanceCancel := context.WithCancel(ctx)
	var maintenance sync.WaitGroup
	maintenance.Add(1)
	go func() {
		defer maintenance.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			if err := db.PruneSecurity(maintenanceCtx, appDB); err != nil && maintenanceCtx.Err() == nil {
				slog.Error("security cleanup failed")
			}
			select {
			case <-maintenanceCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var drain sync.WaitGroup
	for _, server := range servers {
		drain.Add(1)
		go func(server *http.Server) {
			defer drain.Done()
			if err := server.Shutdown(shutdownCtx); err != nil {
				_ = server.Close()
			}
		}(server)
	}
	drain.Wait()
	maintenanceCancel()
	maintenance.Wait()
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
