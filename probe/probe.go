package probe

import (
	"fmt"
	"github.com/nyu-distributed-systems-fa18/BGPalgorand_testMode/pb"
	"golang.org/x/net/context"
	"log"
)

type Probe struct {
	BlockCH chan []*pb.Block
}

func (p *Probe) SendRPState(ctx context.Context, in *pb.RPState) (*pb.Response, error) {
	prettyPrintRPState(in.Message)
	return &pb.Response{Success: "1"}, nil
}

func (p *Probe) SendSLState(ctx context.Context, in *pb.SLState) (*pb.Response, error) {
	prettyPrintSLState(in.Message)
	return &pb.Response{Success: "1"}, nil
}

func (p *Probe) SendBlockChain(ctx context.Context, in *pb.Blockchain) (*pb.Response, error) {
	p.BlockCH <- in.Blocks
	return &pb.Response{Success: "1"}, nil
}

func prettyPrintRPState(State []string) {
	fmt.Printf("/********** round: %s, period: %s ***********/\n", State[0], State[1])
}
func prettyPrintSLState(State []string) {
	log.Printf("> Step: %s, Leader: %s\n", State[0], State[1])
}
