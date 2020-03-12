package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
	"io"
	"os"
	"strings"
	"time"
)

type Peer struct {
	rdb  *redis.Client
	activePeers string
	allPeers string
	host       host.Host
	port       int
	hostname   string
	protocol   string
	dbAddress  string
	rendezVous string
	ctx        context.Context
	privKey    crypto.PrivKey
	keyFile    string
	resetKey   bool
	peerstoreFile string
	peerList   []string
}

func (p *Peer) peerInit() error {
	err := p.redisInit()
	if err != nil {
		fmt.Println("[PEER] Database connection failed -", err)
		return err
	}

	// prepare p2p host
	p.p2pInit(p.keyFile, p.resetKey)

	p.activePeers = p.host.ID().Pretty() + "-active"
	p.allPeers = p.host.ID().Pretty() + "-all"

	// link to a listener for new connections
	// TODO: this can't be tested that easily on localhost, as they will connect to the same db. Perhaps more redises?
	// TODO: this needs access to the db object. It can be global or passed in a function:
	// TODO:     https://stackoverflow.com/questions/26211954/how-do-i-pass-arguments-to-my-handler
	p.host.SetStreamHandler(protocol.ID(p.protocol), p.listener)

	// run peer discovery in the background
	err = p.discoverPeers()
	if err != nil {
		return err
	}

	go p.pingLoop()
	return nil
}

func (p *Peer) redisInit() error {
	// connect to the database
	// TODO: not crashing when database is offline would be nice
	//  also when database shuts down while this is still running...
	p.rdb = redis.NewClient(&redis.Options{
		Addr:     p.dbAddress,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// delete db
	// TODO: remove for production
	p.rdb.FlushAll()

	pongErr := p.rdb.Ping().Err()

	if pongErr != nil {
		fmt.Println("[PEER] Database connection failed -", pongErr)
	}

	return pongErr
}

func (p *Peer) p2pInit(keyFile string, keyReset bool) error {
	p.ctx = context.Background()

	prvKey := p.loadKey(keyFile, keyReset)

	p.privKey = prvKey

	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", p.hostname, p.port))

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
	var err error
	p.host, err = libp2p.New(
		p.ctx,
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)

	if err != nil {
		fmt.Println("[PEER] P2P initialization failed -", err)
		return err
	}

	fmt.Printf("\n[*] Your Multiaddress Is: /ip4/%s/tcp/%v/p2p/%s\n", p.hostname, p.port, p.host.ID().Pretty())
	return nil
}

func (p *Peer) discoverPeers() error {
	fmt.Println("Looking for peers")

	peerChan, err := initMDNS(p.ctx, p.host, p.rendezVous)

	if err != nil {
		return err
	}
	go func() {	for {
		peerAddress := <-peerChan // will block until we discover a peerAddress

		remotePeer := peerAddress.ID
		remotePeerStr := remotePeer.Pretty()
		remoteMA := fmt.Sprintf("%s/p2p/%s", peerAddress.Addrs[0], remotePeerStr)
		p.addNewPeer(remoteMA)
	}}()
	return nil
}

func (p *Peer) talker(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)
	fmt.Println("I am talking now")

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}

func (p *Peer) listener(stream network.Stream) {
	defer p.closeStream(stream)

	remotePeer := stream.Conn().RemotePeer()
	remotePeerStr := remotePeer.Pretty()
	remoteMA := fmt.Sprintf("%s/p2p/%s", stream.Conn().RemoteMultiaddr(), remotePeerStr)
	p.addNewPeer(remoteMA)
	fmt.Println("New stream from", remoteMA)
}

func rw2channel(input chan string, rw *bufio.ReadWriter) {
	for {
		result, err := rw.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Println("err on rw2channel:", err)
		}

		input <- result
	}
}

func send2rw (rw *bufio.ReadWriter, message string) bool {
	_, err := rw.WriteString(message)

	if err != nil {
		return false
	}
	err = rw.Flush()
	if err != nil {
		return false
	}

	return true
}


func (p *Peer) SendAndWait (data string, timeout int) string {
	// for now, use the entire active list
	// TODO: choose 50 peers
	//// TODO: consider broadcasting
	//peerList := p.GetActivePeers()
	//
	//peerstore.AddrBook()
	return ""
}

func (p *Peer) pingLoop() {
	for {
		fmt.Println("[LOOP] printing active peers:")
		for peerID := range p.peerList {
			fmt.Printf("[LOOP] peer %s\n", peerID)
		}
		fmt.Println("[LOOP] done, sleeping 10s")
		time.Sleep(10 * time.Second)
	}
	// sleep
	// for each active peer
	// should i ping?
	// ping
	// too many failures?
	// remove peer from actives
}

func (p *Peer) openStreamFromPeerData(remoteMA string) network.Stream{
	// new multiaddress from string
	multiaddress, err := multiaddr.NewMultiaddr(remoteMA)
	if err != nil {
		fmt.Printf("Error parsing multiaddress '%s': %s\n", remoteMA, err)
		return nil
	}

	// addrInfo from multiaddress
	remotePeer, err := peer.AddrInfoFromP2pAddr(multiaddress)
	if err != nil {
		fmt.Println("Error creating addrInfo from multiaddress:", err)
		return nil
	}

	// open stream
	stream, err := p.host.NewStream(p.ctx, remotePeer.ID, protocol.ID(p.protocol))
	if err != nil {
		fmt.Println("Error opening stream:", err)
		return nil
	}

	return stream
}

func (p *Peer) closeStream(stream network.Stream) {
	if stream == nil {
		// nil streams cause SIGSEGV errs when they are closed
		// fmt.Println("Stream is nil, not closing")
		return
	}
	err := stream.Close()
	if err != nil {
		fmt.Println("Error closing stream")
	}
}

func (p *Peer) sendMessageToStream(stream network.Stream, msg string, timeout time.Duration) (response string, ok bool) {

	// open rw
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	fmt.Printf("Sending message: '%s'\n", msg)
	if !send2rw(rw, msg) {
		fmt.Println("error sending")
		// p.peerstore.decreaseGoodCount(remotePeerStr)
		return "", false
	}

	output := make(chan string)

	go rw2channel(output, rw)
	data := ""
	select{
	case data = <- output:
		break
	case <-time.After(timeout * time.Second):
		fmt.Println("timeout")
	}

	return data, true
}

func (p *Peer) addNewPeer(remoteMA string) {
	if !strings.Contains(strings.Join(p.peerList, ","), remoteMA) {
  		fmt.Println("New peer found")
		p.peerList = append(p.peerList, remoteMA)
	}
}