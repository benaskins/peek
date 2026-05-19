package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func main() {
	flag.Usage = func() { fmt.Fprintf(os.Stderr, "usage: peek <file.md>\n") }
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	if _, err := os.Stat(path); err != nil {
		die("%v", err)
	}

	sidecarPath := path + ".peek.json"
	sc, err := LoadSidecar(sidecarPath)
	if err != nil {
		die("load sidecar: %v", err)
	}
	sc.Source = path

	api := NewServer(sc, ServerOpts{Debounce: 300 * time.Millisecond})

	done := make(chan struct{})
	var shutdownOnce sync.Once
	shutdown := func(reason string) {
		shutdownOnce.Do(func() {
			fmt.Fprintln(os.Stderr, "peek: shutting down ("+reason+")")
			close(done)
		})
	}

	watcher := NewWatcher(WatcherOpts{
		Grace:    30 * time.Second,
		Shutdown: shutdown,
	})

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", newPageHandler(path))
	mux.Handle("GET /static/", http.StripPrefix("/static/", newStaticHandler()))
	mux.Handle("/annotations", api)
	mux.Handle("/annotations/", api)
	mux.HandleFunc("POST /heartbeat", func(w http.ResponseWriter, _ *http.Request) {
		watcher.Beat()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /bye", func(w http.ResponseWriter, _ *http.Request) {
		watcher.Bye()
		w.WriteHeader(http.StatusNoContent)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		die("listen: %v", err)
	}
	url := fmt.Sprintf("http://%s/", listener.Addr())

	httpSrv := &http.Server{Handler: mux}
	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "peek: serve:", err)
		}
	}()
	go watcher.Run(done)

	fmt.Fprintln(os.Stderr, "peek:", url, "(Ctrl+C to stop, or just close the tab)")
	if err := openBrowser(url); err != nil {
		fmt.Fprintln(os.Stderr, "peek: open:", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		shutdown("signal")
	}()

	<-done

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	httpSrv.Shutdown(ctx)
	api.Flush()
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "peek: "+format+"\n", args...)
	os.Exit(1)
}
