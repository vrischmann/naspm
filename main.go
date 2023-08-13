package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	"tailscale.com/tsnet"
)

const nasMACAddress = `6C:BF:B5:02:55:08`

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to execute command, err: %w", err)
	}

	if len(output) > 0 {
		log.Printf("command output: %s", string(output))
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
		flListenAddr = flag.String("listen-addr", ":4790", "The address to listen on")
		flMode       = flag.String("mode", "sleeper", "Which mode to run in. 'sleeper' or 'waker'")
	)
	flag.Parse()

	// Prepare the TCP listener on the tailnet

	hostname := fmt.Sprintf("mynas-%s", *flMode)

	tsserver := &tsnet.Server{
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
