// DigitalisationPM — a local project-management application for
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
	"time"

	"digipm/internal/db"
	"digipm/internal/store"
	"digipm/internal/web"
)

const version = "1.1.0"

func main() {
	port := flag.Int("port", 8383, "port to listen on (0 = random free port)")
	noBrowser := flag.Bool("no-browser", false, "do not open the browser automatically")
	dataDirFlag := flag.String("data", "", "data directory (default: 'data' beside the executable)")
	flag.Parse()

	dataDir := *dataDirFlag
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatal("create data directory %s: %v", dataDir, err)
	}

	// Log to file and stderr; with -H windowsgui stderr goes nowhere, so
	// the file is the record.
	logFile, err := os.OpenFile(filepath.Join(dataDir, "app.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		defer logFile.Close()
	}
	log.Printf("DigitalisationPM v%s starting; data in %s", version, dataDir)

	dbPath := filepath.Join(dataDir, "app.db")
	sqldb, err := db.Open(dbPath)
	if err != nil {
		fatal("open database: %v", err)
	}
	defer sqldb.Close()

	st := store.New(sqldb)

	backupDir := filepath.Join(dataDir, "backups")
	if path, err := st.AutoBackup(backupDir, 14); err != nil {
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
		// Port taken (perhaps the app is already running) — fall back to
		// a random free port rather than failing.
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fatal("listen: %v", err)
		}
	}
	url := fmt.Sprintf("http://%s", listener.Addr().String())
	log.Printf("listening on %s", url)

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if !*noBrowser {
			// Give the server a beat to accept connections.
			time.Sleep(150 * time.Millisecond)
			openBrowser(url)
		}
	}()

	// Graceful shutdown on Ctrl+C / termination.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Println("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		fatal("server: %v", err)
	}
	log.Println("stopped")
}

// defaultDataDir prefers a 'data' folder beside the executable (portable
// install); if that location is not writable it falls back to LocalAppData.
func defaultDataDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "data")
		if writable(dir) {
			return dir
		}
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "DigitalisationPM", "data")
	}
	return "data"
}

func writable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe := filepath.Join(dir, ".write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return false
	}
	os.Remove(probe)
	return true
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

func fatal(format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(1)
}
