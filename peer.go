package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
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
	activePeers   string
	allPeers      string
	host          host.Host
	port          int
	hostname      string
	protocol      string
	dbAddress     string
	rendezVous    string
	ctx           context.Context
	privKey       crypto.PrivKey
	keyFile       string
	resetKey      bool
	peerstoreFile string
	peerList      map[string]*PeerData
	streamQueue   []*network.Stream
	streamNameQueue   []string
}

func (p *Peer) peerInit() error {
	// prepare p2p host
	p.p2pInit(p.keyFile, p.resetKey)

	p.activePeers = p.host.ID().Pretty() + "-active"
	p.allPeers = p.host.ID().Pretty() + "-all"

	p.peerList = make(map[string]*PeerData)
	p.streamQueue = make([]*network.Stream, 0)

	// link to a listener for new connections
	// TODO: this can't be tested that easily on localhost, as they will connect to the same db. Perhaps more redises?
	// TODO: this needs access to the db object. It can be global or passed in a function:
	// TODO:     https://stackoverflow.com/questions/26211954/how-do-i-pass-arguments-to-my-handler
	p.host.SetStreamHandler(protocol.ID(p.protocol), p.listener)

	// run peer discovery in the background
	err := p.discoverPeers()
	if err != nil {
		return err
	}

	go p.talker()
	return nil
}

func (p *Peer) p2pInit(keyFile string, keyReset bool) error {
	p.ctx = context.Background()

	r := rand.Reader
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		fmt.Printf("[KEY UTIL] Error generating key - %s\n", err)
		return nil
	}

	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", p.hostname, p.port))

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
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

		if err := p.host.Connect(p.ctx, peerAddress); err != nil {
			fmt.Println("Connection failed:", err)
			return
		}

		p.openStreamFromPeerData(remoteMA)

		addNewPeer(&p.peerList, remoteMA)
	}}()
	return nil
}

func (p *Peer) talker() {
	stdReader := bufio.NewReader(os.Stdin)
	fmt.Println("I am talking now")

	for {

		if len(p.streamQueue) > 0 {
			stream := p.streamQueue[0]
			name := p.streamNameQueue[0]
			p.streamQueue = p.streamQueue[1:]
			p.streamNameQueue = p.streamNameQueue[1:]

			remotePeer := p.peerList[strings.ToLower(name)]
			fmt.Printf("New stream from %s, message: %s\n", name, remotePeer.Messages)

			fmt.Print("Accept? [y/n] ")
			command, err := stdReader.ReadString('\n')
			if err != nil {
				fmt.Println("Error reading from stdin")
				panic(err)
			}

			command = strings.ToLower(strings.TrimSpace(command))

			if command == "n" {
				fmt.Println("Closing stream")
				(*stream).Close()
				continue
			}

			// connect to stream
			chatWithPeer(stream, remotePeer)
			fmt.Println("Closing stream")
			(*stream).Close()
			continue
		}

		fmt.Print("> ")
		command, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		command = strings.TrimSpace(command)
		parsedCommand := strings.Split(command, " ")

		if parsedCommand[0] == "ls" {
			// list peers
			fmt.Println("Available peers")
			for _, data := range p.peerList {
				fmt.Printf("%s, address: %s\n", data.Name, data.MultiAddress)
			}
		} else if parsedCommand[0] == "open"{
			if len(parsedCommand) < 2 {
				fmt.Println("Please provide a node name")
				continue
			}
			name := strings.ToLower(parsedCommand[1])
			peerData, ok := p.peerList[name]
			if !ok {
				fmt.Println("There is no peer named", name)
				continue
			}
			fmt.Println("Opening stream with", name)

			stream := p.openStreamFromPeerData(peerData.MultiAddress)
			if stream == nil {
				fmt.Println("Couldn't open stream", peerData.MultiAddress)
				continue
			}
			chatWithPeer(&stream, peerData)
		} else {
			fmt.Printf("Unknown command: '%s'\nUse 'ls' or 'open name'\n", command)
		}
	}
}

func (p *Peer) listener(stream network.Stream) {
	remotePeer := stream.Conn().RemotePeer()
	remotePeerStr := remotePeer.Pretty()
	remoteMA := fmt.Sprintf("%s/p2p/%s", stream.Conn().RemoteMultiaddr(), remotePeerStr)
	peerData := addNewPeer(&p.peerList, remoteMA)
	fmt.Println("New stream from", remoteMA)
	p.streamQueue = append(p.streamQueue, &stream)
	p.streamNameQueue = append(p.streamNameQueue, peerData.Name)
}

func chatWithPeer(stream *network.Stream, remotePeer *PeerData) {
	// open rw
	rw := bufio.NewReadWriter(bufio.NewReader(*stream), bufio.NewWriter(*stream))
	// open read from stdin
	stdReader := bufio.NewReadWriter(bufio.NewReader(os.Stdin), nil)

	fmt.Printf("Chatting with %s. What do you want to say?\n", remotePeer.Name)

	ch := make(chan string)
	go readData(rw, remotePeer.Name, ch)
	go readData(stdReader, "user", ch)
	var data string

	for {
		data =<- ch

		fmt.Printf("Data is '%s'\n", data)

		dataSplit := strings.SplitN(data, " ", 2)
		source := dataSplit[0]
		message := strings.TrimSpace(dataSplit[1])

		fmt.Printf("Data split: user '%s', message: '%s'\n", source, message)

		if message == "x" {
			fmt.Println("Closing chat...")
			err := (*stream).Close()
			if err == nil{
				fmt.Println("Error closing")
			}
			return
		}

		if source == "user"{
			_, err := rw.WriteString(fmt.Sprintf("%s\n", message))
			if err != nil {
				fmt.Println("Error writing to buffer")
				panic(err)
			}
			err = rw.Flush()
			if err != nil {
				fmt.Println("Error flushing buffer")
				panic(err)
			}
		} else {
			fmt.Println("unknown source")
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m: %s\n", source, message)
		}
	}
}

func readData(rw *bufio.ReadWriter, name string, ch chan string) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			ch <- fmt.Sprintf("%s x", name)
			return
		}

		if str == "" {
			return
		}
		if str != "\n" {
			ch <- fmt.Sprintf("%s %s", name, str)
		}
	}
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
