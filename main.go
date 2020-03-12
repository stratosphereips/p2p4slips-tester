package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	help := flag.Bool("help", false, "Display Help")
	cfg := parseFlags()

	if *help {
		fmt.Println("This is the P2P component of the Stratosphere Linux IPS.")
		fmt.Println("Run './p2p-experiments' to start it.")
		fmt.Println("For testing multiple peers on one machine, use './p2p-experiments -port [port]'")

		fmt.Println()
		fmt.Println("Usage:")
		flag.PrintDefaults()

		os.Exit(0)
	}

	name, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	fmt.Println("hostname:", name)

	keyFile := cfg.keyFile
	peerstoreFile := cfg.peerstoreFile
	if cfg.addPortToFilename {
		if keyFile != "" {
			keyFile = fmt.Sprintf("%s%d", keyFile, cfg.listenPort)
		}
		if peerstoreFile != "" {
			peerstoreFile = fmt.Sprintf("%s%d", peerstoreFile, cfg.listenPort)
		}
	}

	peer := Peer{
		dbAddress:cfg.redisDb,
		port:cfg.listenPort,
		protocol:cfg.ProtocolID,
		hostname:cfg.listenHost,
		rendezVous:cfg.RendezvousString,
		peerstoreFile:peerstoreFile,
		keyFile:keyFile,
		resetKey:cfg.resetKeys,
	}

	err = peer.peerInit()

	if err != nil {
		fmt.Println("Initializing peer failed")
		os.Exit(1)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)

	<- ch
	fmt.Printf("\nReceived signal, shutting down...\n")

	os.Exit(0)
}
