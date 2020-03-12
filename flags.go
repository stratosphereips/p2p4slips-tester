package main

import (
	"flag"
)

type config struct {
	RendezvousString  string
	ProtocolID        string
	keyFile           string
	peerstoreFile     string
	addPortToFilename bool
	listenHost        string
	listenPort        int
	resetKeys         bool
	redisDb           string
	redisChannel      string
}

func parseFlags() *config {
	c := &config{}

	flag.StringVar(&c.RendezvousString, "rendezvous", "slips", "Unique string to identify group " +
		"of nodes. Share this with your friends to let them connect with you")
	flag.StringVar(&c.listenHost, "host", "0.0.0.0", "The bootstrap node host listen address\n")
	flag.StringVar(&c.ProtocolID, "pid", "/slips/1.0", "Sets a protocol id for stream headers")
	flag.IntVar(&c.listenPort, "port", 4001, "node listen port")

	flag.StringVar(&c.keyFile, "key-file", "", "File containing keys. If it is provided, keys " +
		"will be loaded from the file and saved to it for later use. If no file is specified, one time keys will be " +
		"generated")
	flag.BoolVar(&c.resetKeys, "key-reset", false, "Delete old keys and create new ones")

	flag.StringVar(&c.peerstoreFile, "peerstore-file", "", "File containing known peers. If it is" +
		" provided, peers will be loaded from the file and saved to it for later use. If no file is specified, or if " +
		"the file cannot be decrypted with the given private key, empty peerstore will be created")

	flag.BoolVar(&c.addPortToFilename, "add-port-to-filename", true, "Port is appended to " +
		"filenames for convenient running of more peers on one host. Set to true to keep filenames unchanged")

	flag.StringVar(&c.redisDb, "redis-db", "localhost:6379", "Remote redis database")
	flag.StringVar(&c.redisChannel, "redis-channel", "gotest", "Channel for listening to commands")

	flag.Parse()
	return c
}
