package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vrischmann/hutil/v3"
	"golang.org/x/exp/slices"
	"tailscale.com/tsnet"

	"go.rischmann.fr/naspm/assets"
	"go.rischmann.fr/naspm/ui"
)

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

func wakeOnLan(macAddr string) error {
	return runCommand("wol", macAddr)
}

func sleepHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost || req.URL.Path != "/do" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := powerOff(); err != nil {
		log.Printf("unable to power off, err: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}

func newWakupHandler(macAddress string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/do" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if err := wakeOnLan(macAddress); err != nil {
			log.Printf("unable to power off, err: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})
}

type uiHandler struct {
	basePath        string
	tsHTTPClient    *http.Client
	wakerHostname   string
	sleeperHostname string
}

func newUIHandler(basePath string, tsHTTPCLient *http.Client, wakerHostname string, sleeperHostname string) http.Handler {
	res := &uiHandler{
		basePath:        basePath,
		tsHTTPClient:    tsHTTPCLient,
		wakerHostname:   wakerHostname,
		sleeperHostname: sleeperHostname,
	}

	return res
}

func (h *uiHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	head, _ := hutil.ShiftPath(req.URL.Path)
	if head == "assets" {
		assets.FileServer.ServeHTTP(w, req)
		return
	}

	switch req.Method {
	case http.MethodGet:
		h.renderUI(w, req)
	case http.MethodPost:
		h.handleForm(w, req)
	}
}

type uiStatus struct {
	Status  string
	Message string
}

func unmarshalUIStatus(s string) (res uiStatus) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return res
	}

	_ = json.Unmarshal(data, &res)

	return res
}

func marshalUIStatus(status uiStatus) string {
	data, _ := json.Marshal(status)
	s := base64.StdEncoding.EncodeToString(data)

	return s
}

func uiStatusFromErr(err error) (res uiStatus) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		res.Status = "failure"
		res.Message = "Request timed out"
	case err != nil:
		res.Status = "failure"
		res.Message = "Request failed"
	default:
		res.Status = "success"
		res.Message = "Request succeeded"
	}

	return res
}

const uiCookieName = "mynas-status"

func (h *uiHandler) renderUI(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Check if there's a status cookie

	var status uiStatus
	statusCookie, err := req.Cookie(uiCookieName)
	if err == nil && statusCookie != nil {
		status = unmarshalUIStatus(statusCookie.Value)
	}

	// Clear the status cookie if necessary

	http.SetCookie(w, &http.Cookie{
		Name:   uiCookieName,
		Path:   h.basePath,
		MaxAge: -1,
	})

	// Render

	log.Printf("===> rendering UI, status: %q, message: %q", status.Status, status.Message)

	index := ui.Index(h.basePath, ui.Status{
		Status:  status.Status,
		Message: status.Message,
	})

	w.WriteHeader(http.StatusOK)
	index.Render(req.Context(), w)
}

func (h *uiHandler) handleForm(w http.ResponseWriter, req *http.Request) {
	log.Printf("===> handling UI form")

	// Parse the form and do the appropriate action

	if err := req.ParseForm(); err != nil {
		log.Printf("unable to parse the form, err: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var err error
	if _, ok := req.Form["wakeup"]; ok {
		err = h.doReq(req.Context(), h.wakerHostname)
	} else if _, ok = req.Form["sleep"]; ok {
		err = h.doReq(req.Context(), h.sleeperHostname)
	}

	if err != nil {
		log.Printf("unable to send wakeup or sleep request, err: %v", err)
	}

	// Set the status in a cookie

	status := uiStatusFromErr(err)

	http.SetCookie(w, &http.Cookie{
		Name:  uiCookieName,
		Path:  h.basePath,
		Value: marshalUIStatus(status),
	})

	http.Redirect(w, req, h.basePath, http.StatusSeeOther)
}

func (h *uiHandler) doReq(ctx context.Context, hostname string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	//

	u := fmt.Sprintf("http://%s/do", hostname)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return fmt.Errorf("unable to create request, err: %w", err)
	}

	resp, err := h.tsHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to do POST request, err: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func main() {
	var (
		flTSNetDir             = flag.String("tsnet-dir", "", "The directory where the tsnet state is stored")
		flListenAddr           = flag.String("listen-addr", ":4790", "The address to listen on")
		flMode                 = flag.String("mode", os.Getenv("MODE"), "Which mode to run in. 'sleeper', 'waker' or 'ui'")
		flMACAddress           = flag.String("mac-address", os.Getenv("MAC_ADDRESS"), "The MAC address of the device to wake up. Mantadory if mode is 'waker'")
		flWakerHostname        = flag.String("waker-hostname", os.Getenv("WAKER_HOSTNAME"), "The hostname of the 'waker' device. Mandatory if mode is 'ui'")
		flSleeperHostname      = flag.String("sleeper-hostname", os.Getenv("SLEEPER_HOSTNAME"), "The hostname of the 'sleeper' device. Mandatory if mode is 'ui'")
		flBasePath             = flag.String("base-path", os.Getenv("BASE_PATH"), "The base path for the UI URLs. Useful if the UI is behind a reverse proxy")
		flAuthorizedLoginNames = flag.String("authorized-login-names", os.Getenv("AUTHORIZED_LOGIN_NAMES"), "A comma-separated list of login names that are authorized to access the service")
	)
	flag.Parse()

	if *flTSNetDir == "" {
		log.Fatal("Please provide the directory for the tsnet library state with --tsnet-dir. See --help")
	}

	if *flMode == "waker" && *flMACAddress == "" {
		log.Fatal("Please provide the MAC address to wake up with --mac-address. See --help")
	}

	if *flMode == "ui" && (*flWakerHostname == "" || *flSleeperHostname == "") {
		log.Fatal("Please provide the waker and sleeper hostname with --waker-hostname and --sleeper-hostname. See --help")
	}

	authorizedLoginNames := strings.Split(*flAuthorizedLoginNames, ",")
	if len(authorizedLoginNames) <= 0 {
		log.Fatal("Please provde the authorized login names with with --authorized-login-names. See --help")
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

	var ms hutil.MiddlewareStack

	ms.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			who, err := tsclient.WhoIs(req.Context(), req.RemoteAddr)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if slices.Contains(authorizedLoginNames, who.UserProfile.LoginName) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, req)
		})
	})

	var handler http.Handler
	switch *flMode {
	case "sleeper":
		handler = http.HandlerFunc(sleepHandler)
	case "waker":
		handler = newWakupHandler(*flMACAddress)
	case "ui":
		handler = newUIHandler(
			*flBasePath,
			tsserver.HTTPClient(),
			*flWakerHostname,
			*flSleeperHostname,
		)
	}

	http.Serve(ln, ms.Handler(handler))
}
