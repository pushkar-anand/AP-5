package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pushkar-anand/ap-5/internal/config"
	"github.com/pushkar-anand/ap-5/internal/secrets"
)

func authCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: ap5 auth <subcommand>")
		fmt.Fprintln(os.Stderr, "  set-jn66-token <email>    Store JN-66 bearer token for a Gmail account")
		fmt.Fprintln(os.Stderr, "  set-gmail-credentials     Store Gmail OAuth2 client ID and secret")
		os.Exit(1)
	}

	switch args[0] {
	case "set-jn66-token":
		authSetJN66Token(args[1:])
	case "set-gmail-credentials":
		authSetGmailCredentials(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown auth subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func authSetJN66Token(args []string) {
	fs := flag.NewFlagSet("set-jn66-token", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "directory for secrets")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: ap5 auth set-jn66-token <email>")
		os.Exit(1)
	}
	email := fs.Arg(0)

	store, err := secrets.NewStore(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open secret store: %v\n", err)
		os.Exit(1)
	}

	token := promptSecret(fmt.Sprintf("Enter JN-66 bearer token for %s: ", email))

	if err := store.Set(secrets.JN66TokenKey(email), token); err != nil {
		fmt.Fprintf(os.Stderr, "failed to store token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("JN-66 token stored for %s\n", email)
}

func authSetGmailCredentials(args []string) {
	fs := flag.NewFlagSet("set-gmail-credentials", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath(), "path to config.yaml")
	dataDir := fs.String("data", defaultDataDir(), "directory for secrets")
	_ = fs.Parse(args)

	_, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	store, err := secrets.NewStore(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open secret store: %v\n", err)
		os.Exit(1)
	}

	clientID := promptSecret("Enter Gmail OAuth2 client ID: ")
	clientSecret := promptSecret("Enter Gmail OAuth2 client secret: ")

	if err := store.Set("ap5/gmail/client_id", clientID); err != nil {
		fmt.Fprintf(os.Stderr, "failed to store client ID: %v\n", err)
		os.Exit(1)
	}
	if err := store.Set("ap5/gmail/client_secret", clientSecret); err != nil {
		fmt.Fprintf(os.Stderr, "failed to store client secret: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Gmail OAuth2 credentials stored.")
}

func promptSecret(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}
