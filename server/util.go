package main

import (
	"bytes"
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nyu-distributed-systems-fa18/BGPalgorand_testMode/pb"
)

const BLOCK_SIZE = 10000

func verifyProposal(round int64, period int64, credence int64, q []byte, lastHash string, message *pb.ProposeBlockArgs, pubKey *rsa.PublicKey) bool {
	//log.Printf("round:%d,period:%d,credence:%d,Q:%v,hash:%s", message.Round, message.Period, message.Credence, message.Q, message.HashOfLastBlock)
	//log.Printf("round:%d,period:%d,credence:%d,Q:%v,hash:%s", round, period, credence, q, lastHash)
	if round != message.Round || period != message.Period || credence != message.Credence || !bytes.Equal(q, message.Q) || lastHash != message.HashOfLastBlock {
		return false
	}
	if calculateHash(message.Block) != message.HashOfCurrentBlock {
		return false
	}
	sigParams := []string{strconv.FormatInt(round, 10),
		strconv.FormatInt(period, 10),
		"Sharding",
		strconv.FormatInt(credence, 10),
		string(q),
		lastHash,
		message.HashOfCurrentBlock,
	}
	if err := VerifyMessage(sigParams, pubKey, message.Signature.SignedMessage); err != nil {
		return false
	}
	return true
}

func verifyTransaction(localTxs map[int64]*pb.Transaction, recievedTxs []*pb.Transaction) bool {
	for _, t := range recievedTxs {
		if _, ok := localTxs[t.Id]; ok {
			continue
		} else {
			return false
		}
	}
	return true
}

func verifyVote(round int64, period int64, message *pb.VoteArgs, pubkey *rsa.PublicKey) bool {
	if round != message.Round || period != message.Period {
		return false
	}
	sigParams := []string{message.Value, strconv.FormatInt(round, 10),
		strconv.FormatInt(period, 10),
		message.VoteType,
	}

	if err := VerifyMessage(sigParams, pubkey, message.Signature.SignedMessage); err != nil {
		return false
	}
	return true
}

func hashByteTo01(message []byte) float64 {
	h := sha256.New()
	h.Write(message)
	hashed := h.Sum(nil)
	data := float64(binary.BigEndian.Uint64(hashed))
	x := find(data)
	return x
}

func calculateHash(block *pb.Block) string {
	var transactions bytes.Buffer
	for _, tx := range block.Tx {
		transactions.WriteString(tx.V)
	}
	record := string(block.Id) + block.Timestamp + transactions.String() + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func find(data float64) float64 {
	for i := float64(10); i > 0; i = i * 10 {
		x := data / i
		if x < 1 {
			return x
		}
	}
	return 1
}

func prepareBlock(transactions map[int64]*pb.Transaction, blockchain []*pb.Block) *pb.Block {
	newBlock := new(pb.Block)
	lastBlock := blockchain[len(blockchain)-1]

	newBlock.Id = lastBlock.Id + 1
	newBlock.PrevHash = lastBlock.Hash

	//copy Transactions over to new Block
	newBlock.Tx = []*pb.Transaction{}

	blockMap := make(map[int64]bool)
	// loop through lastBlock's transactions and remove any that appear in newBlock
	for _, tx := range lastBlock.Tx {
		blockMap[tx.Id] = true
	}

	for k, _ := range blockMap {
		if _, ok := transactions[k]; ok {
			delete(transactions, k)
		}
	}

	tempTx := []*pb.Transaction{}
	if len(transactions) < BLOCK_SIZE {
		for _, v := range transactions {
			tempTx = append(tempTx, v)
		}
	} else {
		i := 0
		for _, v := range transactions {
			tempTx = append(tempTx, v)
			if i == 999 {
				break
			}
			i++
		}
	}

	newBlock.Tx = tempTx

	newBlock.Timestamp = time.Now().String()
	newBlock.Hash = calculateHash(newBlock)

	return newBlock
}

func initCredence(userIds []string) map[string]int64 {
	idTocredence := make(map[string]int64)
	for _, id := range userIds {
		idTocredence[id] = int64(1)
	}
	return idTocredence
}

func isPotentialLeader(sig *pb.SIGRet) bool {
	h := hashByteTo01(sig.SignedMessage)
	if h > 0 && h < 0.15 {
		return true
	}
	return false
}

func calculateNextQ(Q string, nextRound int64, key *rsa.PrivateKey) []byte {
	newQ, err := SignMessage([]string{Q, strconv.FormatInt(nextRound, 10)}, key)
	if err != nil {
		log.Panic(err)
	}
	return newQ
}

func SIG(i string, message []string, key *rsa.PrivateKey) *pb.SIGRet {
	signedMessage, err := SignMessage(message, key)
	if err != nil {
		log.Panic(err)
	}
	return &pb.SIGRet{UserId: i, SignedMessage: signedMessage}
}

func selectLeader(proposedValues map[string]string) string {
	minCredential := ""
	leader := ""

	for k, v := range proposedValues {
		if minCredential == "" {
			minCredential = k
			leader = v
		} else if k < minCredential {
			minCredential = k
			leader = v
		}
	}

	return leader
}

func PrettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		return string(b)
	}
	return ""
}

//not used in this system
func GenKeyPair(ID string) error {
	_dir := "./" + ID
	err := os.Mkdir(_dir, os.ModePerm)
	//generate the private key
	privateKey, err := rsa.GenerateKey(crand.Reader, 512)
	if err != nil {
		return err
	}

	der := x509.MarshalPKCS1PrivateKey(privateKey)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: der,
	}
	file, err := os.Create(_dir + "/private.pem")
	if err != nil {
		return err
	}
	err = pem.Encode(file, block)
	if err != nil {
		return nil
	}
	//generate the public key
	public := &privateKey.PublicKey
	def, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return err
	}

	block = &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: def,
	}
	file, err = os.Create(_dir + "/public.pem")
	if err != nil {
		return err
	}
	err = pem.Encode(file, block)
	if err != nil {
		return err
	}
	return nil
}

func Getpubkeyfrombyte(userid string) *rsa.PublicKey {
	publ, err := ioutil.ReadFile("./" + userid + "/" + "public.pem")
	block, _ := pem.Decode(publ)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		log.Fatal("failed to decode PEM block containing public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)

	if err != nil {
		log.Fatal(err)
	}
	return pub.(*rsa.PublicKey)
}

func Getprikeyfrombyte(userid string) *rsa.PrivateKey {
	priv, err := ioutil.ReadFile("./" + userid + "/" + "private.pem")
	if err != nil {
		log.Panic(err)
	}
	block, _ := pem.Decode(priv)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Fatal("failed to decode PEM block containing public key")
	}
	pri, err := x509.ParsePKCS1PrivateKey(block.Bytes)

	if err != nil {
		log.Fatal(err)
	}
	return pri
}

func GenerateKeyPair(userID []string) (map[string]*rsa.PrivateKey, map[string]*rsa.PublicKey) {
	idToPrivateKey := make(map[string]*rsa.PrivateKey)
	idToPublicKey := make(map[string]*rsa.PublicKey)

	for _, v := range userID {
		//generate the private key
		privateKey, err := rsa.GenerateKey(crand.Reader, 512)
		if err != nil {
			log.Panic("generate key error")
		}
		//generate the public key
		publicKey := &privateKey.PublicKey
		idToPrivateKey[v] = privateKey
		idToPublicKey[v] = publicKey
	}

	return idToPrivateKey, idToPublicKey
}

func SignMessage(message []string, Prikey *rsa.PrivateKey) ([]byte, error) {
	s := strings.Join(message[:], "")

	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)

	opts := rsa.PSSOptions{rsa.PSSSaltLengthAuto, crypto.SHA256}
	sig, err := rsa.SignPSS(crand.Reader, Prikey, crypto.SHA256, hashed, &opts)
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func VerifyMessage(message []string, Pubkey *rsa.PublicKey, sig []byte) error {
	s := strings.Join(message[:], "")

	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)

	opts := rsa.PSSOptions{rsa.PSSSaltLengthAuto, crypto.SHA256}
	err := rsa.VerifyPSS(Pubkey, crypto.SHA256, hashed, sig, &opts)
	if err != nil {
		return err
	}
	return nil
}

func isCommitteeMember(user string, committeeMember []string) bool {
	for _, i := range committeeMember {
		if user == i {
			return true
		}
	}
	return false
}

func executeProposal(state *ServerState, proposal *pb.ProposeBlockArgs, bcs *BCStore) {
	//log.Printf("%v", PrettyPrint(pbc.arg.Block))
	proposerId := proposal.Signature.UserId
	sigValue := string(proposal.Signature.SignedMessage)
	//log.Printf("ProposeBlock from %v", proposerId)

	//Has proposaled?
	if _, ok := state.periodState.idToBlock[proposerId]; ok {
		log.Printf("%s have proposaled in this period", proposerId)
		return
	}
	verified := verifyProposal(state.round, state.period, state.idToCredence[proposerId],
		state.Q, bcs.blockchain[len(bcs.blockchain)-1].Hash, proposal, state.idToPublicKey[proposerId])
	verifiedTX := verifyTransaction(state.transactionPool, proposal.Block.Tx)

	if verified && verifiedTX {
		// add verified block to list of blocks I've seen this period
		state.committeeMember = append(state.committeeMember, proposerId)
		state.periodState.sigLeaderToId[sigValue] = proposerId
		state.periodState.idToBlock[proposerId] = proposal.Block
		state.periodState.valueToQ[proposal.HashOfCurrentBlock] = proposal.NextQ
		state.periodState.valueToBlock[proposal.HashOfCurrentBlock] = proposal.Block
	} else {
		// rejected proposed block
		log.Printf("DENIED that %v is on the committee for round %v", proposerId, state.round)
	}
}
