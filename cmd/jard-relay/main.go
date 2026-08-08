// Command jard-relay carries a sandbox's egress to the proxy on the host.
//
// It exists because of where the proxy has to live. Policy and, later, stored
// credentials belong to the host daemon — a container cannot read the OS
// keychain — but a sandbox on an internal network has no route to the host at
// all. This runs in a container attached to both, and forwards to exactly one
// address.
//
// It is deliberately incurious: it reads no bytes, makes no decisions, and can
// reach nowhere but the proxy. Everything that decides anything is on the far
// side of it. See docs/concessions.md for why this piece exists.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var version = "dev"

func main() { os.Exit(run()) }

func run() int {
	var (
		listen      string
		upstream    string
		showVersion bool
	)
	flag.StringVar(&listen, "listen", ":8080", "address to accept sandbox connections on")
	flag.StringVar(&upstream, "upstream", "", "the host proxy to forward to, as host:port")
	flag.BoolVar(&showVersion, "version", false, "print the version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("jard-relay", version)
		return 0
	}
	if upstream == "" {
		log.Println("jard-relay: -upstream is required")
		return 2
	}

	lis, err := net.Listen("tcp", listen)
	if err != nil {
		log.Printf("jard-relay: listening on %s: %v", listen, err)
		return 1
	}
	defer func() { _ = lis.Close() }()
	log.Printf("jard-relay: %s -> %s", listen, upstream)

	// closing the listener is what ends Accept, so the signal handler does
	// that rather than exiting out from under connections in flight.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		_ = lis.Close()
	}()

	for {
		conn, err := lis.Accept()
		if err != nil {
			return 0 // the listener closed, which is how this stops
		}
		go forward(conn, upstream)
	}
}

// forward joins one accepted connection to a fresh upstream one.
func forward(client net.Conn, upstream string) {
	defer func() { _ = client.Close() }()

	server, err := net.Dial("tcp", upstream)
	if err != nil {
		log.Printf("jard-relay: dialling %s: %v", upstream, err)
		return
	}
	defer func() { _ = server.Close() }()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); copyThrough(server, client) }()
	go func() { defer wg.Done(); copyThrough(client, server) }()
	wg.Wait()
}

// copyThrough moves bytes one way and then half-closes, so the far side sees
// the end of the stream instead of waiting on it.
func copyThrough(dst, src net.Conn) {
	_, _ = io.Copy(dst, src)
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
