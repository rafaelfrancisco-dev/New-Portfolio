// revoke-sessions invalidates all sessions and pending OAuth requests locally.
package main

import (
	"context"
	"go-port/internal/db"
	"log"
	"os"
)

func main() {
	path := os.Getenv("APP_DB")
	if path == "" {
		log.Fatal("APP_DB is required")
	}
	d, err := db.Connect(path)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()
	if err = db.RevokeSessions(context.Background(), d); err != nil {
		log.Fatal(err)
	}
	log.Print("all sessions revoked")
}
