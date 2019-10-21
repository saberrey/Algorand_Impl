package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/saberrey/BGPalgorand_testMode/pb"
	"github.com/saberrey/BGPalgorand_testMode/probe"
)

const TRANSACTION_NUMBER = 1000

func PrettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		return string(b)
	}
	return ""
}

func updateEndTimeMap(end map[int64]time.Time, b []*pb.Block) map[int64]time.Time {
	lastblock := b[len(b)-1]
	for _, tx := range lastblock.Tx {
		if _, ok := end[tx.Id]; !ok {
			end[tx.Id] = time.Now()
		}
	}
	return end
}

func getTimeDurationMap(begin map[int64]time.Time, end map[int64]time.Time) map[int64]float64 {
	result := make(map[int64]float64)
	for k, v := range begin {
		result[k] = end[k].Sub(v).Seconds()
	}
	return result
}
func calculateDuration(result map[int64]float64) (float64, float64, float64) {
	avg := float64(0)
	max := float64(0)
	min := float64(10000)
	for _, v := range result {
		if max < v {
			max = v
		}
		if min > v {
			min = v
		}
		avg = avg + v
	}
	return max, min, avg / float64(len(result))
}

func usage() {
	fmt.Printf("Usage %s <message>\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	// Take endpoint as input
	flag.Usage = usage
	flag.Parse()
	// If there is no endpoint fail
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	/********************Create client****************************/
	fmt.Printf("> booststrap client...")
	bc := make(chan []*pb.Block)
	pr := probe.Probe{bc}
	s := grpc.NewServer()
	pb.RegisterProbeServer(s, &pr)
	c, err := net.Listen("tcp", "127.0.0.1:9900")
	if err != nil {
		// Note the use of Fatalf which will exit the program after reporting the error.
		log.Fatalf("Could not create listening socket %v", err)
	}
	go func() {
		fmt.Printf("done...\n")
		if err := s.Serve(c); err != nil {
			log.Fatalf("Failed to serve %v", err)
		}
	}()
	/*************************************************************/

	sendTransactionsTimer := time.NewTimer(10000 * time.Millisecond)
	begin := make(map[int64]time.Time)
	end := make(map[int64]time.Time)

	for {
		select {
		case blocks := <-bc:
			log.Printf("Chain: %v", PrettyPrint(blocks))
			end := updateEndTimeMap(end, blocks)
			if len(end) == TRANSACTION_NUMBER {
				result := getTimeDurationMap(begin, end)
				log.Printf("> Duration result")
				i, a, v := calculateDuration(result)
				log.Printf("max:%f, min:%f, average:%f", i, a, v)
			}
		case <-sendTransactionsTimer.C:
			endpoint := flag.Args()[0]
			log.Printf("Connecting to %v", endpoint)

			// Connect to the server. We use WithInsecure since we do not configure https in this class.
			conn, err := grpc.Dial(endpoint, grpc.WithInsecure())

			//Ensure connection did not fail.
			if err != nil {
				log.Fatalf("Failed to dial GRPC server %v", err)
			}
			log.Printf("Connected")
			bcc := pb.NewBCStoreClient(conn)

			for i := 0; i < TRANSACTION_NUMBER; i++ {
				transReq := &pb.Transaction{Id: int64(i), V: "yue are good at blockchain"}
				res, err := bcc.Send(context.Background(), transReq)
				begin[int64(i)] = time.Now()
				if err != nil {
					log.Fatalln("send error")
				}
				if err != nil {
					log.Println(err)
				}
				log.Printf("%v", res)
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
