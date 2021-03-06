package neoutils_test

import (
	"log"
	"sort"
	"testing"

	"github.com/o3labs/neo-utils/neoutils"
	"github.com/o3labs/neo-utils/neoutils/btckey"
)

func TestGenerateMultiSigAddress(t *testing.T) {

	pb1 := "02e77ff280db51ef3638009f11947c544ed094d4e5f2d96a9e654dc817bc3a8986"
	pb2 := "024da93f9a66981e499b36ce763e57fd89a47a052e86d40b42f81708c40fe9eff0"
	require := 2
	pubKeys := [][]byte{}

	pubKeys = append(pubKeys, neoutils.HexTobytes(pb1))
	pubKeys = append(pubKeys, neoutils.HexTobytes(pb2))

	multisign := neoutils.MultiSig{}
	vmCode, err := multisign.CreateMultiSigRedeemScript(require, pubKeys)
	if err != nil {
		log.Printf("%v", err)
	}
	log.Printf("vm code %x", vmCode)

	multisigAddress := neoutils.VMCodeToNEOAddress(vmCode)
	log.Printf("multi sig address %v", multisigAddress)
	if multisigAddress != "AKo8k27H5nCG8MwSirmnraH6uUG6fQQVC2" {
		t.Fail()
	}
}

func TestSortPublicKeys(t *testing.T) {
	p1Hex := "02e77ff280db51ef3638009f11947c544ed094d4e5f2d96a9e654dc817bc3a8986"
	p2Hex := "024da93f9a66981e499b36ce763e57fd89a47a052e86d40b42f81708c40fe9eff0"
	p3Hex := "035ca1deea29ccb25a3a4d32701a0e735f76f3b44d233e23930cd74b68a63d10c3"
	p1 := btckey.PublicKey{}
	p2 := btckey.PublicKey{}
	p3 := btckey.PublicKey{}
	p1.FromBytes(neoutils.HexTobytes(p1Hex))
	p2.FromBytes(neoutils.HexTobytes(p2Hex))
	p3.FromBytes(neoutils.HexTobytes(p3Hex))

	keys := []btckey.PublicKey{p3, p1, p2}

	//https://golang.org/pkg/math/big/#Int.Cmp
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].Point.X.Cmp(keys[j].Point.X) == -1 })
	for _, k := range keys {
		log.Printf("%x", k.ToBytes())
	}
	//correct order is p2, p3, p1

}
