package main

import (
	"fmt"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	users := []string{"10", "20"}
	pri, pub := GenerateKeyPair(users)
	message := []string{"yue", "jia", "rui"}
	sm, err := SignMessage(message, pri["1"])
	fmt.Println(sm, err)
	fmt.Println(VerifyMessage(message, pub["1"], sm))
}

func TestGetKeyPair(t *testing.T) {
	GenKeyPair("8003")
	pub := Getpubkeyfrombyte("8003")
	pri := Getprikeyfrombyte("8003")
	message := []string{"yue", "jia", "rui"}
	sm, err := SignMessage(message, pri)
	fmt.Println(sm, err)
	fmt.Println(VerifyMessage(message, pub, sm))
}
func TestHashByteTo01(t *testing.T) {
	fmt.Println(hashByteTo01([]byte("yuejiarui")))
}
