package main

import (
	"crypto/rsa"
	"fmt"
	"github.com/nyu-distributed-systems-fa18/BGPalgorand_testMode/pb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"time"
)

const (
	ROUND_TIMEER         = 6000
	STEP1_TO_STEP4_TIMER = 12000
	STEP5_TIMER          = 2000
	RECEND_TIMER         = 1000
	PROBE_ENDPOINT       = "127.0.0.1:9900"
)

// @idToPrivateKey    A map, userID to its private key.
// @idToPublicKey     A map, userID to its public key
// @idToCredence      A map, userID to its credence(信誉值).
// @Q                 The seed changed every round.
// @round             The round this peer is running in.
// @period            The period this peer is running in.
// @step              The step this peer is running in.
// @readyForNextRound A flag to determine whether to go next round
// @tempBlock		  A buffer to create new block.
// @proposedBlock     The new block proposed by the peer in this @round.
// @periodState       This @period state.
// @lastPeriodState   Last period state.
type ServerState struct {
	idToPrivateKey    map[string]*rsa.PrivateKey
	idToPublicKey     map[string]*rsa.PublicKey
	idToCredence      map[string]int64
	transactionPool   map[int64]*pb.Transaction
	Q                 []byte
	round             int64
	period            int64
	step              int64
	readyForNextRound bool
	committeeMember   []string
	isCommitteeMember bool

	proposedBlock   *pb.Block
	periodState     PeriodState
	lastPeriodState PeriodState
	probe           pb.ProbeClient
	probeConnected  bool
}

// @sigLeaderToId         A map, signature to userID, the signature is used to select leader and committee member.
// @idToBlock             A map, userID to its proposed block.
// @valeToQ               A map, Hash of block to its sigQ, the leader's sigQ will be used as new Q in next round, every peer will send it because they don't know who is leader in this round.
// @valueToBlock  		  A map, The HASH of block proposed by peer to this block.
// @nextVotes             counter of NEXT vote.
// @softVotes             counter of SOFT vote.
// @certVotes             counter of CERT vote.
// @haveNextVotedStep4    A map to determine whether the peer has received the next vote that sent by a same peer in STEP 4.
// @haveNextVotedStep5	  A map to determine whether the peer has received the next vote that sent by a same peer in STEP 5.
// @haveSoftVoted         A map to determine whether the peer has sent soft vote.
// @haveCertVoted         A map to determine whether the peer has sent cert vote.
// @myCertVote            Have I cert a value?
// @leader                The leader in this round.
// @startingValue         The starting value.
// @period                This period number.
type PeriodState struct {
	sigLeaderToId      map[string]string
	idToBlock          map[string]*pb.Block
	valueToQ           map[string][]byte
	valueToBlock       map[string]*pb.Block
	nextVotes          map[string]int64
	softVotes          map[string]int64
	certVotes          map[string]int64
	haveNextVotedStep4 map[string]bool
	haveNextVotedStep5 map[string]bool
	haveSoftVoted      map[string]bool
	haveCertVoted      map[string]bool
	haveChecked        bool
	myCertVote         string
	leader             string
	startingValue      string
	period             int64
}

type AppendBlockInput struct {
	arg      *pb.AppendBlockArgs
	response chan pb.AppendBlockRet
}

type AppendTransactionInput struct {
	arg      *pb.AppendTransactionArgs
	response chan pb.AppendTransactionRet
}

type ProposeBlockInput struct {
	arg      *pb.ProposeBlockArgs
	response chan pb.ProposeBlockRet
}

type VoteInput struct {
	arg      *pb.VoteArgs
	response chan pb.VoteRet
}

type RequestBlockChainInput struct {
	arg      *pb.RequestBlockChainArgs
	response chan pb.RequestBlockChainRet
}

type NoticeFinalBlockValueInput struct {
	arg      *pb.NoticeFinalBlockValueArgs
	response chan pb.NoticeFinalBlockValueRet
}

type Algorand struct {
	AppendBlockChan           chan AppendBlockInput
	AppendTransactionChan     chan AppendTransactionInput
	ProposeBlockChan          chan ProposeBlockInput
	VoteChan                  chan VoteInput
	RequestBlockChainChan     chan RequestBlockChainInput
	NoticeFinalBlockValueChan chan NoticeFinalBlockValueInput
}

func (a *Algorand) AppendBlock(ctx context.Context, arg *pb.AppendBlockArgs) (*pb.AppendBlockRet, error) {
	c := make(chan pb.AppendBlockRet)
	a.AppendBlockChan <- AppendBlockInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) AppendTransaction(ctx context.Context, arg *pb.AppendTransactionArgs) (*pb.AppendTransactionRet, error) {
	c := make(chan pb.AppendTransactionRet)
	a.AppendTransactionChan <- AppendTransactionInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) Vote(ctx context.Context, arg *pb.VoteArgs) (*pb.VoteRet, error) {
	c := make(chan pb.VoteRet)
	a.VoteChan <- VoteInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) ProposeBlock(ctx context.Context, arg *pb.ProposeBlockArgs) (*pb.ProposeBlockRet, error) {
	c := make(chan pb.ProposeBlockRet)
	a.ProposeBlockChan <- ProposeBlockInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) RequestBlockChain(ctx context.Context, arg *pb.RequestBlockChainArgs) (*pb.RequestBlockChainRet, error) {
	c := make(chan pb.RequestBlockChainRet)
	a.RequestBlockChainChan <- RequestBlockChainInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func (a *Algorand) NoticeFinalBlockValue(ctx context.Context, arg *pb.NoticeFinalBlockValueArgs) (*pb.NoticeFinalBlockValueRet, error) {
	c := make(chan pb.NoticeFinalBlockValueRet)
	a.NoticeFinalBlockValueChan <- NoticeFinalBlockValueInput{arg: arg, response: c}
	result := <-c
	return &result, nil
}

func RunAlgorandServer(algorand *Algorand, port int) {
	portString := fmt.Sprintf(":%d", port)

	c, err := net.Listen("tcp", portString)
	if err != nil {
		log.Fatalf("Could not create listening socket %v", err)
	}

	s := grpc.NewServer()

	pb.RegisterAlgorandServer(s, algorand)
	log.Printf("Going to listen on port %v", port)

	if err := s.Serve(c); err != nil {
		log.Fatalf("Failed to serve %v", err)
	}
}

func connectToPeer(peer string) (pb.AlgorandClient, error) {
	backoffConfig := grpc.DefaultBackoffConfig
	// Choose an aggressive backoff strategy here.
	backoffConfig.MaxDelay = 500 * time.Millisecond
	conn, err := grpc.Dial(peer, grpc.WithInsecure(), grpc.WithBackoffConfig(backoffConfig))
	// Ensure connection did not fail, which should not happen since this happens in the background
	if err != nil {
		return pb.NewAlgorandClient(nil), err
	}
	return pb.NewAlgorandClient(conn), nil
}

func restartTimer(timer *time.Timer, ms int64) {
	stopped := timer.Stop()

	if !stopped {
		for len(timer.C) > 0 {
			<-timer.C
		}
	}
	timer.Reset(time.Duration(ms) * time.Millisecond)
}

func initPeriodState(p int64) PeriodState {
	newPeriodState := PeriodState{
		sigLeaderToId:      make(map[string]string),
		idToBlock:          make(map[string]*pb.Block),
		valueToQ:           make(map[string][]byte),
		valueToBlock:       make(map[string]*pb.Block),
		nextVotes:          make(map[string]int64),
		softVotes:          make(map[string]int64),
		certVotes:          make(map[string]int64),
		haveNextVotedStep4: make(map[string]bool),
		haveNextVotedStep5: make(map[string]bool),
		haveSoftVoted:      make(map[string]bool),
		haveCertVoted:      make(map[string]bool),
		haveChecked:        false,
		myCertVote:         "",
		leader:             "",
		startingValue:      "",
		period:             p,
	}
	return newPeriodState
}

func handleHalt(bcs *BCStore, state *ServerState, newBlock *pb.Block, haltValue string) {
	log.Printf("AGREEMENT!")
	bcs.blockchain = append(bcs.blockchain, newBlock)
	//log.Printf("Chain: %v", PrettyPrint(bcs.blockchain))
	if state.probeConnected {
		_, err := state.probe.SendBlockChain(context.Background(), &pb.Blockchain{Blocks: bcs.blockchain})
		if err != nil {
			log.Println("send state to probe endpoint error")
		}
	}
	state.Q = state.periodState.valueToQ[haltValue]

	// Handle Halting Condition
	state.readyForNextRound = true
	state.round++

	state.lastPeriodState = PeriodState{}
	state.period = int64(1)
	state.step = int64(1)
	state.periodState = initPeriodState(state.period)
	state.periodState.startingValue = "_|_"
	state.committeeMember = []string{}
	state.isCommitteeMember = false
	//time.Sleep(time.Millisecond * 2000)
}

func handleNextPeriod(state *ServerState, nextValue string) {
	log.Printf("Enter next period!")

	// finish period
	state.period++
	state.step = 1
	state.lastPeriodState = state.periodState
	state.periodState = initPeriodState(state.period)
	if nextValue != "_|_" {
		state.periodState.startingValue = nextValue
	}

	// allow step1 to happen again
	state.readyForNextRound = true
}

// The main service loop.
func serve(bcs *BCStore, peers *arrayPeers, id string, port int, Q string) {
	log.Printf("All peers: %v", peers)

	algorand := Algorand{
		AppendBlockChan:           make(chan AppendBlockInput),
		AppendTransactionChan:     make(chan AppendTransactionInput),
		ProposeBlockChan:          make(chan ProposeBlockInput),
		VoteChan:                  make(chan VoteInput),
		RequestBlockChainChan:     make(chan RequestBlockChainInput),
		NoticeFinalBlockValueChan: make(chan NoticeFinalBlockValueInput),
	}

	// Start in a Go routine so it doesn't affect us.
	go RunAlgorandServer(&algorand, port)

	// Initial state.
	state := ServerState{
		idToPrivateKey:    make(map[string]*rsa.PrivateKey),
		idToPublicKey:     make(map[string]*rsa.PublicKey),
		idToCredence:      make(map[string]int64),
		transactionPool:   make(map[int64]*pb.Transaction),
		Q:                 []byte(Q), // Q in the paper
		readyForNextRound: true,
		committeeMember:   []string{},
		round:             1,
	}

	peerClients := make(map[string]pb.AlgorandClient)
	peerCount := int64(0)
	userIds := make([]string, len(*peers)+1)

	for i, peer := range *peers {
		client, err := connectToPeer(peer)
		if err != nil {
			log.Fatalf("Failed to connect to GRPC server %v", err)
		}

		peerClients[peer] = client

		userIds[i] = peer

		peerCount++
		log.Printf("Connected to %v", peer)
	}

	if peerCount < 3 {
		log.Fatalf("Need at least 4 nodes to achieve Byzantine fault tolerance")
	}

	//requiredVotes is the required minimum vote number
	var requiredVotes int64

	//proposalBuffer is used to pre-recieve proposal of next round or period
	var proposalBuffer []*pb.ProposeBlockArgs

	// add my Id to pool of userIds
	userId := id
	userIds[len(userIds)-1] = id

	// save the key pair to RAM
	for _, i := range userIds {
		state.idToPrivateKey[i] = Getprikeyfrombyte(i)
		state.idToPublicKey[i] = Getpubkeyfrombyte(i)
	}

	// sort userIds so all Algorand servers have the same list of userIds
	sort.Strings(userIds)
	log.Printf("UserIds: %v", userIds)

	type AppendBlockResponse struct {
		ret  *pb.AppendBlockRet
		err  error
		peer string
	}

	type AppendTransactionResponse struct {
		ret  *pb.AppendTransactionRet
		err  error
		peer string
	}

	type ProposeBlockResponse struct {
		ret   *pb.ProposeBlockRet
		err   error
		peer  string
		block *pb.ProposeBlockArgs
	}

	type VoteResponse struct {
		ret  *pb.VoteRet
		err  error
		peer string
		vote *pb.VoteArgs
	}

	type RequestBlockChainResponse struct {
		ret  *pb.RequestBlockChainRet
		err  error
		peer string
	}

	type NoticeFinalBlockValueResponse struct {
		ret   *pb.NoticeFinalBlockValueRet
		err   error
		peer  string
		value *pb.NoticeFinalBlockValueArgs
	}

	appendBlockResponseChan := make(chan AppendBlockResponse)
	appendTransactionResponseChan := make(chan AppendTransactionResponse)
	proposeBlockResponseChan := make(chan ProposeBlockResponse)
	voteResponseChan := make(chan VoteResponse)
	requestBlockChainResponseChan := make(chan RequestBlockChainResponse)
	noticeFinalBlockValueResponseChan := make(chan NoticeFinalBlockValueResponse)

	// Set timer to check for new rounds
	roundTimer := time.NewTimer(ROUND_TIMEER * time.Millisecond)
	agreementTimer := time.NewTimer(STEP1_TO_STEP4_TIMER * time.Millisecond)
	probeGrpcDialTimer := time.NewTimer(RECEND_TIMER * time.Millisecond)

	// Initial credence
	state.idToCredence = initCredence(userIds)

	//Prepare periodState
	state.period = int64(1)
	state.step = int64(1)
	state.periodState = initPeriodState(state.period)
	state.periodState.startingValue = "_|_"

	// Run forever handling inputs from various channels
	for {
		select {
		case <-probeGrpcDialTimer.C:
			if userId != "ubuntu:8000" {
				break
			}
			go func() {
				conn, err := grpc.Dial(PROBE_ENDPOINT, grpc.WithInsecure())
				//Ensure connection did not fail.
				if err != nil {
					log.Printf("Dial probe endpoint failed, please check if you start it.\n")
					restartTimer(probeGrpcDialTimer, RECEND_TIMER)
				} else {
					log.Printf("connect probe endpoint!")
					state.probe = pb.NewProbeClient(conn)
					state.probeConnected = true
				}
			}()
		case <-roundTimer.C:
			// propose block if last round complete or very first round
			if state.readyForNextRound {
				log.Printf("Starting round %v, period %v", state.round, state.period)

				//execute proposals in proposalBuffer
				for _, p := range proposalBuffer {
					executeProposal(&state, p, bcs)
				}
				proposalBuffer = []*pb.ProposeBlockArgs{}

				//send round&&period info to client
				if state.probeConnected {
					_, err := state.probe.SendRPState(context.Background(), &pb.RPState{Message: []string{strconv.FormatInt(state.round, 10), strconv.FormatInt(state.period, 10)}})
					if err != nil {
						log.Println("send state to probe endpoint error")
					}
				}

				//set the flag
				state.readyForNextRound = false

				// we don't want step two to happen too quick before users can collect proposedBlocks
				restartTimer(agreementTimer, STEP1_TO_STEP4_TIMER)

				// Its not graceful here, but I don't know how to alter
				if value := runStep1(state, requiredVotes); value == "err" {
					panic("no bug, no here")
				} else if value == "" {
					state.proposedBlock = prepareBlock(state.transactionPool, bcs.blockchain)
				} else {
					state.proposedBlock = state.lastPeriodState.valueToBlock[value]
					log.Printf("Recieve 2t+1 next-vote != _|_, proposal value: %v", value)
				}

				var b *pb.Block
				var v string
				var q []byte
				b = state.proposedBlock
				v = calculateHash(state.proposedBlock)
				q = state.Q

				// each server needs exact same seed per round so they all see the same selection
				sigParams := []string{strconv.FormatInt(state.round, 10),
					strconv.FormatInt(state.period, 10),
					"Sharding",
					strconv.FormatInt(state.idToCredence[userId], 10),
					string(q),
					bcs.blockchain[len(bcs.blockchain)-1].Hash,
					v,
				}

				// create block proposal message and signature
				sig := SIG(userId, sigParams, state.idToPrivateKey[userId])

				// If I am a potential leader?
				if isPotentialLeader(sig) {
					state.isCommitteeMember = true
					sigQ := calculateNextQ(string(q), state.round+1, state.idToPrivateKey[userId])
					state.periodState.sigLeaderToId[string(sig.SignedMessage)] = userId
					state.periodState.idToBlock[userId] = b
					state.periodState.valueToQ[v] = sigQ
					state.periodState.valueToBlock[v] = b

					//value
					var round int64
					var period int64
					round = state.round
					period = state.period

					// broadcast proposal
					for p, c := range peerClients {
						go func(c pb.AlgorandClient, p string, b *pb.Block, sig *pb.SIGRet, period int64, round int64, cred int64, lasthash string, sigQ []byte) {
							//log.Printf("Sent proposal to peer %v", p)
							block := &pb.ProposeBlockArgs{
								Peer:               userId,
								Round:              round,
								Period:             period,
								Flag:               "Sharding",
								Credence:           cred,
								Q:                  q,
								HashOfLastBlock:    lasthash,
								NextQ:              sigQ,
								HashOfCurrentBlock: v,
								Block:              b,
								Signature:          sig,
							}
							ret, err := c.ProposeBlock(context.Background(), block)
							proposeBlockResponseChan <- ProposeBlockResponse{ret: ret, err: err, peer: p, block: block}
						}(c, p, b, sig, period, round, state.idToCredence[userId], bcs.blockchain[len(bcs.blockchain)-1].Hash, sigQ)
					}
				}
			}
			restartTimer(roundTimer, ROUND_TIMEER)

		case <-agreementTimer.C:
			//if not committee member, break.
			if state.readyForNextRound == true {
				break
			}

			if !state.isCommitteeMember {
				//although is not committee, should know who is leader to acknowledge the final block value is sent by leader
				state.periodState.leader = selectLeader(state.periodState.sigLeaderToId)
				log.Printf("committee member: %v", len(state.committeeMember))
				break
			}

			// if we are currently in agreement protocol
			if state.step < 5 {
				state.step++
			}
			if state.step == 2 && !state.periodState.haveSoftVoted[userId] {
				//calculate requiredVotes in step 2
				t := (len(state.committeeMember) + 1) / 3
				requiredVotes = int64(2*t + 1)

				log.Printf("STEP 2")
				softVoteV := runStep2(&state.periodState, &state.lastPeriodState, requiredVotes)
				log.Printf("soft vote is %v", softVoteV)
				if state.probeConnected {
					_, err := state.probe.SendSLState(context.Background(), &pb.SLState{Message: []string{strconv.FormatInt(state.step, 10), state.periodState.leader}})
					if err != nil {
						log.Println("send state to probe endpoint error")
					}
				}
				if softVoteV != "" {
					// add my own vote for this value
					state.periodState.softVotes[softVoteV]++
					state.periodState.haveSoftVoted[userId] = true

					// broadcast my decision to vote for this value
					message := []string{
						softVoteV,
						strconv.FormatInt(state.round, 10),
						strconv.FormatInt(state.period, 10),
						"soft",
					}
					softVoteSIG := SIG(userId, message, state.idToPrivateKey[userId])

					//value, maybe it is needed to avoid next round state changed
					var round int64
					var period int64
					var step int64
					round = state.round
					period = state.period
					step = int64(2)
					for _, p := range state.committeeMember {
						c := peerClients[p]
						go func(c pb.AlgorandClient, p string, softVoteSIG *pb.SIGRet, round int64, period int64, softValue string, step int64) {
							//log.Printf("Sent soft vote to peer %v", p)
							vote := &pb.VoteArgs{
								Peer:      userId,
								Value:     softValue,
								Round:     round,
								Period:    period,
								VoteType:  "soft",
								Signature: softVoteSIG,
								Step:      step,
							}
							ret, err := c.Vote(context.Background(), vote)
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p, vote: vote}
						}(c, p, softVoteSIG, round, period, softVoteV, step)
					}
				}
			} else if state.step == 3 && !state.periodState.haveCertVoted[userId] {
				log.Printf("STEP 3")
				certVoteV := runStep3(&state.periodState, requiredVotes)
				log.Printf("cert vote is %v", certVoteV)
				if state.probeConnected {
					_, err := state.probe.SendSLState(context.Background(), &pb.SLState{Message: []string{strconv.FormatInt(state.step, 10), state.periodState.leader}})
					if err != nil {
						log.Println("send state to probe endpoint error")
					}
				}
				if certVoteV != "" {

					// add my own vote for this value
					state.periodState.certVotes[certVoteV]++
					state.periodState.haveCertVoted[userId] = true
					state.periodState.myCertVote = certVoteV

					// broadcast my decision to vote for this value
					message := []string{certVoteV, strconv.FormatInt(state.round, 10),
						strconv.FormatInt(state.period, 10),
						"cert",
					}
					certVoteSIG := SIG(userId, message, state.idToPrivateKey[userId])

					//value, maybe it is needed to avoid next round state changed
					var round int64
					var period int64
					var step int64
					round = state.round
					period = state.period
					step = int64(3)
					for _, p := range state.committeeMember {
						c := peerClients[p]
						go func(c pb.AlgorandClient, p string, certVoteSIG *pb.SIGRet, round int64, certVote string, period int64, step int64) {
							//log.Printf("Sent cert vote to peer %v", p)
							vote := &pb.VoteArgs{
								Peer:      userId,
								Value:     certVote,
								Round:     round,
								Period:    period,
								VoteType:  "cert",
								Signature: certVoteSIG,
								Step:      step,
							}
							ret, err := c.Vote(context.Background(), vote)
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p, vote: vote}
						}(c, p, certVoteSIG, round, certVoteV, period, step)
					}
				}
				haltValue := checkHaltingCondition(&state.periodState, requiredVotes)
				if haltValue != "" {
					if userId == state.periodState.leader {
						for _, i := range userIds {
							if !isCommitteeMember(i, state.committeeMember) && i != userId {
								go func(c pb.AlgorandClient, p string) {
									message := &pb.NoticeFinalBlockValueArgs{Value: haltValue, Peer: p}
									ret, err := c.NoticeFinalBlockValue(context.Background(), message)
									noticeFinalBlockValueResponseChan <- NoticeFinalBlockValueResponse{ret: ret, err: err, peer: p, value: message}
								}(peerClients[i], userId)
							}
						}
					}
					handleHalt(bcs, &state, state.periodState.valueToBlock[haltValue], haltValue)

					//restart round timer to make sure all peers will enter next round
					//if not may round timer is faster than agreement timer,and restart round timer
					//in first case
					restartTimer(roundTimer, 2000)
				}
			} else if state.step == 4 && !state.periodState.haveNextVotedStep4[userId] {
				log.Printf("STEP 4")
				nextVoteV := runStep4(&state.periodState, &state.lastPeriodState, requiredVotes)
				log.Printf("next vote is %v", nextVoteV)
				if state.probeConnected {
					_, err := state.probe.SendSLState(context.Background(), &pb.SLState{Message: []string{strconv.FormatInt(state.step, 10), state.periodState.leader}})
					if err != nil {
						log.Println("send state to probe endpoint error")
					}
				}
				// add my own vote for this value
				state.periodState.nextVotes[nextVoteV]++
				state.periodState.haveNextVotedStep4[userId] = true

				// broadcast my decision to vote for this value
				message := []string{nextVoteV, strconv.FormatInt(state.round, 10),
					strconv.FormatInt(state.period, 10),
					"next",
				}
				nextVoteSIG := SIG(userId, message, state.idToPrivateKey[userId])

				//value, maybe it is needed to avoid next round state changed
				var round int64
				var period int64
				var step int64
				round = state.round
				period = state.period
				step = int64(4)
				for _, p := range state.committeeMember {
					c := peerClients[p]
					go func(c pb.AlgorandClient, p string, nextVoteSIG *pb.SIGRet, round int64, nextVote string, period int64, step int64) {
						//log.Printf("Sent next vote to peer %v", p)
						vote := &pb.VoteArgs{
							Peer:      userId,
							Value:     nextVote,
							Round:     round,
							Period:    period,
							VoteType:  "next",
							Signature: nextVoteSIG,
							Step:      step,
						}
						ret, err := c.Vote(context.Background(), vote)
						voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p, vote: vote}
					}(c, p, nextVoteSIG, round, nextVoteV, period, step)
				}
				nextPeriodStartValue := checkNextPeridCondition(&state.periodState, requiredVotes)
				if nextPeriodStartValue != "" {
					handleNextPeriod(&state, nextPeriodStartValue)
					//restart round timer to make sure all peers will enter next round
					//if not may round timer is faster than agreement timer,and restart round timer
					//in first case
					restartTimer(roundTimer, 2000)
				}

			} else if state.step == 5 && !state.periodState.haveNextVotedStep5[userId] {
				log.Printf("STEP 5")
				nextVoteV := runStep5(&state.periodState, &state.lastPeriodState, requiredVotes)
				log.Printf("next vote is %v", nextVoteV)
				if state.probeConnected {
					_, err := state.probe.SendSLState(context.Background(), &pb.SLState{Message: []string{strconv.FormatInt(state.step, 10), state.periodState.leader}})
					if err != nil {
						log.Println("send state to probe endpoint error")
					}
				}
				if nextVoteV != "" {
					// add my own vote for this value
					state.periodState.nextVotes[nextVoteV]++
					state.periodState.haveNextVotedStep5[userId] = true

					message := []string{nextVoteV, strconv.FormatInt(state.round, 10),
						strconv.FormatInt(state.period, 10),
						"next",
					}
					nextVoteSIG := SIG(userId, message, state.idToPrivateKey[userId])

					//value, maybe it is needed to avoid next round state changed
					var round int64
					var period int64
					var step int64
					round = state.round
					period = state.period
					step = int64(5)
					for _, p := range state.committeeMember {
						c := peerClients[p]
						go func(c pb.AlgorandClient, p string, nextVoteSIG *pb.SIGRet, round int64, nextVote string, period int64, step int64) {
							//log.Printf("Sent next vote to peer %v", p)
							vote := &pb.VoteArgs{
								Peer:      userId,
								Value:     nextVote,
								Round:     round,
								Period:    period,
								VoteType:  "next",
								Signature: nextVoteSIG,
								Step:      step,
							}
							ret, err := c.Vote(context.Background(), vote)
							voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p, vote: vote}
						}(c, p, nextVoteSIG, round, nextVoteV, period, step)
					}
				}
				nextPeriodStartValue := checkNextPeridCondition(&state.periodState, requiredVotes)
				if nextPeriodStartValue != "" {
					handleNextPeriod(&state, nextPeriodStartValue)
					//restart round timer to make sure all peers will enter next round
					//if not may round timer is faster than agreement timer,and restart round timer
					//in first case
					restartTimer(roundTimer, 2000)
				}
			}

			// Handle resetting the agreementTimer
			// we want a shorter timout for continously checking step5 again and again
			if state.step == 5 {
				restartTimer(agreementTimer, STEP5_TIMER)
			} else {
				restartTimer(agreementTimer, STEP1_TO_STEP4_TIMER)
			}

		case op := <-bcs.C:
			// Received a command from client
			// TODO: Add Transaction to our local block, broadcast to every user
			//log.Printf("Transaction request: %#v, Round: %v", op.command.Arg, state.round)
			//op.response <- pb.Result{S: "1"}

			if op.command.Operation == pb.Op_SEND {
				state.transactionPool[op.command.GetTx().Id] = op.command.GetTx()

				// TODO - broadcast, and figure out when to reponse to client? op.response <-(response)
				for p, c := range peerClients {
					transaction := op.command.GetTx()

					go func(c pb.AlgorandClient, p string, transaction *pb.Transaction) {
						//log.Printf("Sent transaction to peer %v", p)
						ret, err := c.AppendTransaction(context.Background(), &pb.AppendTransactionArgs{Peer: p, Tx: transaction})
						appendTransactionResponseChan <- AppendTransactionResponse{ret: ret, err: err, peer: p}
					}(c, p, transaction)
				}
			}
			bcs.HandleCommand(op)
			// Check if add new Transaction, or simply get the curent Blockchain
			// if op.command.Operation == pb.Op_GET {
			// 	log.Printf("Request to view the blockchain")
			// 	bcs.HandleCommand(op)
			// } else {
			// 	log.Printf("Request to add new Block")

			// 	// for now, we simply append to our blockchain and broadcast the new blockchain to all known peers
			// 	bcs.HandleCommand(op)
			// 	for p, c := range peerClients {
			// 		go func(c pb.AlgorandClient, blockchain []*pb.Block, p string) {
			// 			ret, err := c.AppendBlock(context.Background(), &pb.AppendBlockArgs{Blockchain: blockchain, Peer: id})
			// 			appendBlockResponseChan <- AppendResponse{ret: ret, err: err, peer: p}
			// 		}(c, bcs.blockchain, p)
			// 	}
			// 	log.Printf("Period: %v, Blockchain: %#v", state.period, bcs.blockchain)
			// }

		case ab := <-algorand.AppendBlockChan:
			// we got an AppendBlock request
			//log.Printf("AppendBlock from %v", ab.arg.Peer)

			// for now, just check if blockchain is longer than ours
			// if yes, overwrite ours and return true
			// if no, return false
			if len(ab.arg.Blockchain) > len(bcs.blockchain) {
				bcs.blockchain = ab.arg.Blockchain
				ab.response <- pb.AppendBlockRet{Success: true}
			} else {
				ab.response <- pb.AppendBlockRet{Success: false}
			}

		case abr := <-appendBlockResponseChan:
			// we got a response to our AppendBlock request
			//log.Printf("AppendBlockResponse: %#v", abr)
			if abr.err != nil {

			}

		case at := <-algorand.AppendTransactionChan:
			// we got an AppendTransaction request
			//log.Printf("AppendTransaction from %v", at.arg.Peer)

			state.transactionPool[at.arg.Tx.Id] = at.arg.Tx
			log.Printf("Transactions pool size: %d", len(state.transactionPool))

			at.response <- pb.AppendTransactionRet{Success: true}

		case <-appendTransactionResponseChan:
			// we got a response to our AppendTransaction request
			//log.Printf("AppendTransactionResponse: %#v", atr)
		case pbc := <-algorand.ProposeBlockChan:
			if pbc.arg.Round > state.round+1 {
				log.Printf("%v round is behind peers, request blockchains", userId)

				rand.Intn(len(peerClients))
				//random choice one , use the character of map
				k := func(pc map[string]pb.AlgorandClient) string {
					for v, _ := range pc {
						return v
					}
					return ""
				}(peerClients)
				go func(c pb.AlgorandClient, p string) {
					ret, err := c.RequestBlockChain(context.Background(), &pb.RequestBlockChainArgs{Peer: p})
					requestBlockChainResponseChan <- RequestBlockChainResponse{ret: ret, err: err, peer: p}
				}(peerClients[k], k)

				pbc.response <- pb.ProposeBlockRet{Success: false}
				break
			} else if pbc.arg.Round == state.round+1 || pbc.arg.Period == state.period+1 {
				proposalBuffer = append(proposalBuffer, pbc.arg)
				pbc.response <- pb.ProposeBlockRet{Success: true}
				break
			}

			executeProposal(&state, pbc.arg, bcs)
			pbc.response <- pb.ProposeBlockRet{Success: true}

		case pbr := <-proposeBlockResponseChan:
			if (pbr.err != nil || !pbr.ret.Success) && pbr.block.Round == state.round {
				p := pbr.peer
				c := peerClients[p]
				if pbr.block.Period != state.period {
					break
				}
				go func() {
					time.Sleep(time.Millisecond * RECEND_TIMER)
					log.Printf("Resent proposal to peer %v", p)
					ret, err := c.ProposeBlock(context.Background(), pbr.block)
					proposeBlockResponseChan <- ProposeBlockResponse{ret: ret, err: err, peer: p, block: pbr.block}
				}()
			}

		case nbr := <-noticeFinalBlockValueResponseChan:
			if nbr.err != nil || !nbr.ret.Success {
				p := nbr.peer
				c := peerClients[p]
				go func() {
					time.Sleep(time.Millisecond * RECEND_TIMER)
					ret, err := c.NoticeFinalBlockValue(context.Background(), nbr.value)
					noticeFinalBlockValueResponseChan <- NoticeFinalBlockValueResponse{ret: ret, err: err, peer: p, value: nbr.value}
				}()
			}

		case vc := <-algorand.VoteChan:
			//if !state.isCommitteeMember {
			//	log.Printf("Not in committee but recieve vote?")
			//	break
			//}
			if vc.arg.Round > state.round {
				log.Printf("Round is behind peers, request blockchains")

				k := func(pc map[string]pb.AlgorandClient) string {
					for v, _ := range pc {
						return v
					}
					return ""
				}(peerClients)
				go func(c pb.AlgorandClient, p string) {
					ret, err := c.RequestBlockChain(context.Background(), &pb.RequestBlockChainArgs{Peer: p})
					requestBlockChainResponseChan <- RequestBlockChainResponse{ret: ret, err: err, peer: p}
				}(peerClients[k], k)

				vc.response <- pb.VoteRet{Success: false}
				break
			}
			voterId := vc.arg.Peer
			voteValue := vc.arg.Value
			voteType := vc.arg.VoteType
			votePeriod := vc.arg.Period
			//log.Printf("Received %vVote from: %v", voteType, voterId)

			//verify vote signature
			if !verifyVote(state.round, state.period, vc.arg, state.idToPublicKey[voterId]) {
				//log.Println("signature verify wrong, not accept vote")
				break
			}

			if voteType == "soft" {
				_, hasVoted := state.periodState.haveSoftVoted[voterId]
				if !hasVoted {
					if votePeriod == state.periodState.period {
						state.periodState.softVotes[voteValue]++
					} else if votePeriod == state.lastPeriodState.period {
						state.lastPeriodState.softVotes[voteValue]++
					}
					state.periodState.haveSoftVoted[voterId] = true
					vc.response <- pb.VoteRet{Success: true}
				} else {
					log.Printf("Ignoring %vVote from %v: already %vVoted this period", voteType, voterId, voteType)
					vc.response <- pb.VoteRet{Success: false}
				}
			} else if voteType == "cert" {
				_, hasVoted := state.periodState.haveCertVoted[voterId]
				if !hasVoted {
					if votePeriod == state.periodState.period {
						state.periodState.certVotes[voteValue]++
					} else if votePeriod == state.lastPeriodState.period {
						state.lastPeriodState.certVotes[voteValue]++
					}
					state.periodState.haveCertVoted[voterId] = true
					vc.response <- pb.VoteRet{Success: true}
					// we need to check for halting condition anytime we see a new cert vote
					haltValue := checkHaltingCondition(&state.periodState, requiredVotes)
					if haltValue != "" {
						if userId == state.periodState.leader {
							for _, i := range userIds {
								if !isCommitteeMember(i, state.committeeMember) && i != userId {
									go func(c pb.AlgorandClient, p string) {
										message := &pb.NoticeFinalBlockValueArgs{Value: haltValue, Peer: p}
										ret, err := c.NoticeFinalBlockValue(context.Background(), message)
										noticeFinalBlockValueResponseChan <- NoticeFinalBlockValueResponse{ret: ret, err: err, peer: p, value: message}
									}(peerClients[i], userId)
								}
							}
						}
						handleHalt(bcs, &state, state.periodState.valueToBlock[haltValue], haltValue)

						//restart round timer to make sure all peers will enter next round
						//if not may round timer is faster than agreement timer,and restart round timer
						//in first case
						restartTimer(roundTimer, 2000)
					}

				} else {
					log.Printf("Ignoring %vVote from %v: already %vVoted this period", voteType, voterId, voteType)
					vc.response <- pb.VoteRet{Success: false}
				}
			} else if voteType == "next" {
				_, hasVoted4 := state.periodState.haveNextVotedStep4[voterId]
				_, hasVoted5 := state.periodState.haveNextVotedStep5[voterId]
				if !hasVoted4 || !hasVoted5 {
					if votePeriod == state.periodState.period {
						state.periodState.nextVotes[voteValue]++
					} else if votePeriod == state.lastPeriodState.period {
						state.lastPeriodState.nextVotes[voteValue]++
					}
					if vc.arg.Step == int64(4) {
						state.periodState.haveNextVotedStep4[voterId] = true
					} else {
						state.periodState.haveNextVotedStep5[voterId] = true
					}
					vc.response <- pb.VoteRet{Success: true}

					nextPeriodStartValue := checkNextPeridCondition(&state.periodState, requiredVotes)
					if nextPeriodStartValue != "" {
						handleNextPeriod(&state, nextPeriodStartValue)
						//restart round timer to make sure all peers will enter next round
						//if not may round timer is faster than agreement timer,and restart round timer
						//in first case
						restartTimer(roundTimer, 2000)
					}
				} else {
					log.Printf("Ignoring %vVote from %v: already %vVoted this period. step %d", voteType, voterId, voteType, vc.arg.Step)
					vc.response <- pb.VoteRet{Success: false}
				}
			} else {
				log.Printf("Strange to arrive here")
				vc.response <- pb.VoteRet{Success: false}
			}
		case vr := <-voteResponseChan:
			if (vr.err != nil || vr.ret.Success == false) && vr.vote.Round == state.round {
				pe := vr.vote.Period
				if pe != state.period {
					//log.Printf("vote has been expired, %s", vr.peer)
					break
				}
				p := vr.peer
				c := peerClients[p]
				go func() {
					time.Sleep(time.Millisecond * RECEND_TIMER)
					log.Printf("Resent %svote to peer %v", vr.vote.VoteType, p)
					ret, err := c.Vote(context.Background(), vr.vote)
					voteResponseChan <- VoteResponse{ret: ret, err: err, peer: p, vote: vr.vote}
				}()
			}

		case bcc := <-algorand.RequestBlockChainChan:
			log.Printf("RequestBlockChain from: %v", bcc.arg.Peer)

			bcc.response <- pb.RequestBlockChainRet{Peer: userId, Blockchain: bcs.blockchain, Q: state.Q}

		case nfbv := <-algorand.NoticeFinalBlockValueChan:
			if nfbv.arg.Peer != state.periodState.leader {
				log.Println("eerros")
				break
			}
			log.Printf("Noticed block value :%v from leader: %v\n", nfbv.arg.Value, nfbv.arg.Peer)
			handleHalt(bcs, &state, state.periodState.valueToBlock[nfbv.arg.Value], nfbv.arg.Value)
			restartTimer(roundTimer, 2000)
			nfbv.response <- pb.NoticeFinalBlockValueRet{Success: true}

		case bcr := <-requestBlockChainResponseChan:
			log.Printf("Received Blockchain from %v", bcr.peer)

			if bcr.err == nil {
				candidateBlockchain := bcr.ret.Blockchain

				if len(candidateBlockchain) > len(bcs.blockchain) {
					log.Printf("CandidateChain: %v", PrettyPrint(candidateBlockchain))

					// verify every block in this blockchain
					verified := true
					//TODO:delete transcation here.
					if verified {
						log.Printf("Verified new Blockchain from peer: %v", bcr.ret.Peer)
						bcs.blockchain = candidateBlockchain

						// Prepare to reenter into Agreement
						state.readyForNextRound = true
						state.round = int64(len(bcs.blockchain))

						state.lastPeriodState = PeriodState{}
						state.period = int64(1)
						state.step = int64(1)
						state.Q = bcr.ret.Q
						state.periodState = initPeriodState(state.period)
						state.periodState.startingValue = "_|_"
					}

					log.Printf("NewChain: %v", PrettyPrint(bcs.blockchain))
				}
			}

		}
	}
	log.Printf("Strange to arrive here1")
}
