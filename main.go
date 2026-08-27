// Digital Project Management is a local application for
// digitalisation project leaders. Starts a localhost web server backed by
// SQLite and opens the default browser.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/db"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/store"
	"github.com/seanmcmahon101/Digital-Project-Management/internal/web"
)

// These values are injected by the tagged release workflow. Development
// builds remain explicit rather than pretending to be a published release.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	port := flag.Int("port", 8383, "port to listen on (0 = random free port)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically")
	dataDirFlag := flag.String("data", "", "use a specific data directory")
	portable := flag.Bool("portable", false, "store data beside the executable")
	showVersion := flag.Bool("version", false, "print version and build information")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Digital Project Management %s (commit %s, built %s)\n", version, commit, buildDate)
		return
	}
	if *port < 0 || *port > 65535 {
		fatal("port must be between 0 and 65535")
	}
	useDefaultWorkspace := *dataDirFlag == "" && !*portable
	if *port != 0 && useDefaultWorkspace {
		if existingURL, ok := existingInstance(*port); ok {
			log.Printf("Digital Project Management is already running at %s", existingURL)
			if !*noBrowser {
				openBrowser(existingURL)
			}
			return
		}
	}

	dataDir := *dataDirFlag
	if dataDir == "" {
		dataDir = defaultDataDir(*portable)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		fatal("create data directory %s: %v", dataDir, err)
	}
	_ = os.Chmod(dataDir, 0o700)

	// Keep a local log as well as terminal output so startup and recovery
	// problems remain diagnosable after the process exits.
	logFile, err := os.OpenFile(filepath.Join(dataDir, "app.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		defer logFile.Close()
	}
	log.Printf("Digital Project Management %s starting; data in %s", version, dataDir)

	dbPath := filepath.Join(dataDir, "app.db")
	sqldb, err := db.Open(dbPath)
	if err != nil {
		fatal("open database: %v", err)
	}
	_ = os.Chmod(dbPath, 0o600)

	st := store.New(sqldb)
	defer func() { _ = st.DB.Close() }()

	backupDir := filepath.Join(dataDir, "backups")
	if path, err := st.AutoBackupWorkspace(dataDir, backupDir, version, 14); err != nil {
		log.Printf("auto-backup failed: %v", err)
	} else if path != "" {
		log.Printf("auto-backup created: %s", path)
	}

	srv, err := web.NewServer(st, dataDir, backupDir, dbPath, version)
	if err != nil {
		fatal("initialise server: %v", err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		if *port != 0 && useDefaultWorkspace {
			if existingURL, ok := existingInstance(*port); ok {
				log.Printf("Digital Project Management is already running at %s", existingURL)
				if !*noBrowser {
					openBrowser(existingURL)
				}
				return
			}
		}
		fatal("listen on port %d: %v (use -port 0 to choose a free port)", *port, err)
	}
	url := fmt.Sprintf("http://%s", listener.Addr().String())
	log.Printf("listening on %s", url)

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Minute,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		if !*noBrowser {
			// Give the server a beat to accept connections.
			time.Sleep(150 * time.Millisecond)
			openBrowser(url)
		}
	}()

	// Graceful shutdown on Ctrl+C / termination.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}()

	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		fatal("server: %v", err)
	}
	log.Println("stopped")
}

// defaultDataDir uses a stable per-user location. An existing legacy workspace
// beside the executable is preserved automatically; new portable installs must
// opt in so extracting an upgrade elsewhere cannot appear to lose their data.
func defaultDataDir(portable bool) string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "data")
		if portable {
			return dir
		}
		if info, err := os.Stat(filepath.Join(dir, "app.db")); err == nil && !info.IsDir() {
			return dir
		}
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "DigitalisationPM", "data")
	}
	return "data"
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser automatically: %v — open %s manually", err, url)
	}
}

// existingInstance distinguishes this application from an unrelated process
// that happens to occupy the configured port.
func existingInstance(port int) (string, bool) {
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 750 * time.Millisecond}
	response, err := client.Get(url + "/healthz")
	if err != nil {
		return "", false
	}
	defer response.Body.Close()
	return url, response.StatusCode == http.StatusOK &&
		response.Header.Get(web.InstanceHeader) == "DigitalProjectManagement"
}

func fatal(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}
