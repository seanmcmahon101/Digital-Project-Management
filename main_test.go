package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/seanmcmahon101/Digital-Project-Management/internal/web"
)

func TestDefaultDataDirUsesStableUserLocation(t *testing.T) {
	want := filepath.Join("DigitalisationPM", "data")
	got := defaultDataDir(false)
	if filepath.Base(got) != "data" || filepath.Base(filepath.Dir(got)) != "DigitalisationPM" {
		t.Fatalf("defaultDataDir(false) = %q, want suffix %q", got, want)
	}
}

func TestExistingInstanceRequiresApplicationIdentity(t *testing.T) {
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set(web.InstanceHeader, "DigitalProjectManagement")
		w.WriteHeader(http.StatusOK)
	}))
	defer instance.Close()
	_, portValue, err := net.SplitHostPort(instance.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		t.Fatal(err)
	}
	if url, ok := existingInstance(port); !ok || url == "" {
		t.Fatalf("running instance not detected: url=%q ok=%v", url, ok)
	}

	unrelated := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer unrelated.Close()
	_, unrelatedPortValue, _ := net.SplitHostPort(unrelated.Listener.Addr().String())
	unrelatedPort, _ := strconv.Atoi(unrelatedPortValue)
	if _, ok := existingInstance(unrelatedPort); ok {
		t.Fatal("unrelated local service was treated as this application")
	}
}

func TestDefaultDataDirPortableIsBesideExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(exe), "data")
	if got := defaultDataDir(true); got != want {
		t.Fatalf("defaultDataDir(true) = %q, want %q", got, want)
	}
}
