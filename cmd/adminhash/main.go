package main

import (
	"fmt"
	"os"

	"go-port/internal/auth"

	"golang.org/x/term"
)

func main() {
	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read password:", err)
		os.Exit(1)
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read confirmation:", err)
		os.Exit(1)
	}
	if len(first) < 12 {
		fmt.Fprintln(os.Stderr, "password must be at least 12 characters")
		os.Exit(1)
	}
	if !auth.SecureEqual(string(first), string(second)) {
		fmt.Fprintln(os.Stderr, "passwords do not match")
		os.Exit(1)
	}
	hash, err := auth.HashPassword(string(first))
	if err != nil {
		fmt.Fprintln(os.Stderr, "hash password:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
