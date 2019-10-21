//Package main, This package contains all the features except the communication module.
//Here are some details:
//   agreement.go: The consensus logic of Algorand
//   bcStore.go:   Send or Get transactions API
//   main.go:      The entrance of the peer
//   peers.go:     Something useless but MAYBE useful in the future
//   serve.go:     The peer's running process
//   util.go:      Some useful function used in this project, for example: HASH, RSA
//
//   @Author:      Jerry Yue
//   @Date:        2019.8.28
package main

import (
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"sync"

	"github.com/saberrey/BGPalgorand_testMode/pb"
)

func createGenesisBlock() *pb.Block {
	genesisBlock := new(pb.Block)
	genesisBlock.Id = 0
	genesisBlock.Timestamp = "The first year of creation"
	genesisBlock.PrevHash = ""
	genesisBlock.Hash = calculateHash(genesisBlock)
	genesisBlock.Tx = nil
	return genesisBlock
}

func main() {
	var seed string
	var peerNumber int
	var clientPort int
	var algorandPort int
	flag.StringVar(&seed, "seed", "yuejiarui",
		"Seed for random number generator, values less than 0 result in use of time")
	flag.IntVar(&clientPort, "port", 6000,
		"Port on which server should listen to client requests")
	flag.IntVar(&algorandPort, "algorand", 8000,
		"Port on which server should listen to Algorand requests")
	flag.IntVar(&peerNumber, "peerNumber", 50,
		"The number of sharding")
	flag.Parse()

	name, err := os.Hostname()
	if err != nil {
		// Without a host name we can't really get an ID, so die.
		log.Fatalf("Could not get hostname")
	}

	id := fmt.Sprintf("%s:%d", name, algorandPort)
	idEnd := fmt.Sprintf("%s:%d", name, algorandPort+peerNumber-1)
	log.Printf("peers' ID from %s to %s", id, idEnd)

	//get all peers
	var peers arrayPeers

	var algorandPorts []int
	var clientPorts []int

	var bcses []BCStore

	var listeners []net.Listener
	var grpcServers []*grpc.Server
	for i := 0; i < peerNumber; i++ {

		id := fmt.Sprintf("%s:%d", name, algorandPort)
		peers = append(peers, id)
		GenKeyPair(id)

		algorandPorts = append(algorandPorts, algorandPort)
		clientPorts = append(clientPorts, clientPort)

		bcs := BCStore{C: make(chan InputChannelType), blockchain: []*pb.Block{}}
		bcs.blockchain = append(bcs.blockchain, createGenesisBlock())
		bcses = append(bcses, bcs)

		portString := fmt.Sprintf(":%d", clientPort)
		c, err := net.Listen("tcp", portString)
		if err != nil {
			// Note the use of Fatalf which will exit the program after reporting the error.
			log.Fatalf("Could not create listening socket %v", err)
		}
		listeners = append(listeners, c)

		s := grpc.NewServer()
		grpcServers = append(grpcServers, s)

		algorandPort++
		clientPort++
	}

	for i := 0; i < peerNumber; i++ {
		// Spin up algorand server
		peersWithoutMyself := getPeersWithoutMyself(peers, peers[i])

		b := &bcses[i]
		ps := &peersWithoutMyself
		p := peers[i]
		a := algorandPorts[i]
		go serve(b, ps, p, a, seed)

		pb.RegisterBCStoreServer(grpcServers[i], &bcses[i])
		log.Printf("Going to listen on port %v", clientPorts[i])
		// Start serving, this will block this function and only return when done.

		l := listeners[i]
		g := grpcServers[i]
		go func() {
			if err := g.Serve(l); err != nil {
				log.Fatalf("Failed to serve %v", err)
			}
			log.Printf("Done peer %s", peers[i])
		}()

	}

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()

}

func getPeersWithoutMyself(peers arrayPeers, id string) arrayPeers {
	var pwm arrayPeers
	for _, p := range peers {
		if p != id {
			pwm = append(pwm, p)
		}
	}
	return pwm
}
