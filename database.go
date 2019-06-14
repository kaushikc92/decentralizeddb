package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"

	dht "github.com/libp2p/go-libp2p-kad-dht"
)

type databaseconfig struct {
	dbname string
	nNodes int
}

var dbconfig databaseconfig

func handleStream(stream network.Stream) {
	fmt.Println("Got a new stream!")

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	str, _ := rw.ReadString('\n')
	fmt.Printf("\x1b[32m%s\x1b[0m", str)

	if str == "connect\n" {
		connectNode(rw)
	}
}

func connectNode(rw *bufio.ReadWriter) {
	//_, _ = rw.WriteString(fmt.Sprintf("%s %d\n", dbconfig.dbname, dbconfig.n))
	_, _ = rw.WriteString(fmt.Sprintf("%s\n", dbconfig.dbname))
	_, _ = rw.WriteString(fmt.Sprintf("%d\n", dbconfig.nNodes))
	_ = rw.Flush()

	str, _ := rw.ReadString('\n')
	fmt.Printf("\x1b[32m%s\x1b[0m", str)
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

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

func announceDatabase(host host.Host, routingDiscovery *discovery.RoutingDiscovery) {
	host.SetStreamHandler(protocol.ID("/picolo/1.0"), handleStream)
	fmt.Println("Announcing ourselves...")
	discovery.Advertise(context.Background(), routingDiscovery, "Database Bootstrap")
	fmt.Println("Successfully announced!")
}

func connectToDatabase(host host.Host, routingDiscovery *discovery.RoutingDiscovery) {
	fmt.Println("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(context.Background(), "Database Bootstrap")
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}
		fmt.Println("Found peer:", peer)

		fmt.Println("Connecting to:", peer)
		stream, err := host.NewStream(context.Background(), peer.ID, protocol.ID("/picolo/1.0"))

		if err != nil {
			fmt.Println("Connection failed:", err)
			continue
		} else {
			rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

			_, _ = rw.WriteString(fmt.Sprintf("%s\n", "connect"))
			_ = rw.Flush()

			str, _ := rw.ReadString('\n')
			fmt.Printf("\x1b[32m%s\x1b[0m", str)

			str, _ = rw.ReadString('\n')
			fmt.Printf("\x1b[32m%s\x1b[0m", str)

			_, _ = rw.WriteString(fmt.Sprintf("%s\n", peer.ID))
			_ = rw.Flush()

			fmt.Println("Connected to:", peer)
			break
		}
	}
}

func main() {
	sourcePort := flag.Int("p", 3001, "Port number")
	mode := flag.String("m", "announce", "Mode")
	dbname := flag.String("d", "mydb", "Database Name")
	nNodes := flag.Int("n", 1, "Number of nodes")

	flag.Parse()

	dbconfig.dbname = *dbname
	dbconfig.nNodes = *nNodes

	ctx := context.Background()
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", *sourcePort)),
	)
	if err != nil {
		panic(err)
	}

	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		panic(err)
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
			} else {
				fmt.Println("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)

	if *mode == "connect" {
		connectToDatabase(host, routingDiscovery)
	} else {
		announceDatabase(host, routingDiscovery)
	}

	select {}
}
