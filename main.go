package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"tailscale.com/tsnet"
)

const nasMACAddress = `6C:BF:B5:02:55:08`

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin

	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		log.Printf("command output: %s", string(output))
	}

	if err != nil {
		return fmt.Errorf("unable to execute command, err: %w", err)
	}
	return nil
}

func powerOff() error {
	return runCommand("systemctl", "poweroff")
}

func wakeOnLan() error {
	return runCommand("wol", nasMACAddress)
}

func main() {
	var (
		flTSNetDir   = flag.String("tsnet-dir", "", "The directory where the tsnet state is stored")
		flListenAddr = flag.String("listen-addr", ":4790", "The address to listen on")
		flMode       = flag.String("mode", "sleeper", "Which mode to run in. 'sleeper' or 'waker'")
	)
	flag.Parse()

	if *flTSNetDir == "" {
		log.Fatal("Please provide the directory for the tsnet library state")
	}

	// Prepare the TCP listener on the tailnet

	hostname := fmt.Sprintf("mynas-%s", *flMode)

	tsserver := &tsnet.Server{
		Dir:      filepath.Join(*flTSNetDir, hostname),
		Hostname: hostname,
	}
	defer tsserver.Close()

	ln, err := tsserver.Listen("tcp", *flListenAddr)
	if err != nil {
		log.Fatalf("unable to listen on the tailnet, err: %v", err)
	}

	tsclient, err := tsserver.LocalClient()
	if err != nil {
		log.Fatalf("unable to get local tailscale client, err: %v", err)
	}

	// Setup the HTTP handler

	http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		who, err := tsclient.WhoIs(req.Context(), req.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if who.UserProfile.LoginName != "vrischmann@github" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if req.Method != http.MethodPost || req.URL.Path != "/do" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		switch *flMode {
		case "sleeper":
			err = powerOff()
		case "waker":
			err = wakeOnLan()
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	}))
}
