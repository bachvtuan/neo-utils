package smartcontract

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/apisit/btckeygenie/btckey"
)

type ScriptHash []byte
type NEOAddress []byte

func (s ScriptHash) ToString() string {
	return hex.EncodeToString(s)
}
func ParseNEOAddress(address string) NEOAddress {
	v, b, _ := btckey.B58checkdecode(address)
	if v != 0x17 {
		return nil
	}
	return NEOAddress(b)
}

type ScriptBuilderInterface interface {
	generateContractInvocationScript(scriptHash ScriptHash, operation string, args []interface{}) []byte
	generateTransactionAttributes(attributes map[TransactionAttribute][]byte) ([]byte, error)
	generateTransactionInput(unspent Unspent, assetToSend NativeAsset, amountToSend float64) ([]byte, error)
	generateTransactionOutput() ([]byte, error)

	ToBytes() []byte
	FullHexString() string
	pushInt(value int) error
	pushData(data interface{}) error
	Clear()
	pushLength(count int)
}

func NewScriptBuilder() ScriptBuilderInterface {
	return &ScriptBuilder{RawBytes: []byte{}}
}

type ScriptBuilder struct {
	RawBytes []byte
}

func (s ScriptBuilder) ToBytes() []byte {
	return s.RawBytes
}

func (s *ScriptBuilder) Clear() {
	s.RawBytes = []byte{}
}

func (s ScriptBuilder) FullHexString() string {
	b := s.ToBytes()
	return hex.EncodeToString(b)
}

func (s *ScriptBuilder) pushOpCode(opcode OpCode) {
	s.RawBytes = append(s.RawBytes, byte(opcode))
}

func (s *ScriptBuilder) pushInt(value int) error {
	switch {
	case value == -1:
		s.pushOpCode(PUSHM1)
		return nil
	case value == 0:
		s.pushOpCode(PUSH0)
		return nil
	case value >= 1 && value < 16:
		rawValue := byte(PUSH1) + byte(value) - 1
		s.RawBytes = append(s.RawBytes, rawValue)
		return nil
	case value >= 16:
		num := make([]byte, 8)
		binary.LittleEndian.PutUint64(num, uint64(value))
		s.RawBytes = append(s.RawBytes, bytes.TrimRight(num, "\x00")...)
		return nil
	}
	return nil
}

func (s *ScriptBuilder) pushLength(count int) {
	countBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(countBytes, uint64(count))
	trimmedCountByte := bytes.TrimRight(countBytes, "\x00")
	s.RawBytes = append(s.RawBytes, trimmedCountByte...)
}

func (s *ScriptBuilder) pushHexString(hexString string) error {
	b, err := hex.DecodeString(hexString)
	if err != nil {
		return err
	}
	count := len(b)
	countBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(countBytes, uint64(count))
	trimmedCountByte := bytes.TrimRight(countBytes, "\x00")

	if count < int(PUSHBYTES75) {
		s.RawBytes = append(s.RawBytes, trimmedCountByte...)
		s.RawBytes = append(s.RawBytes, b...)
	} else if count < 0x100 {
		s.pushOpCode(PUSHDATA1)
		s.RawBytes = append(s.RawBytes, trimmedCountByte...)
		s.RawBytes = append(s.RawBytes, b...)
	} else if count < 0x10000 {
		s.pushOpCode(PUSHDATA2)
		s.RawBytes = append(s.RawBytes, trimmedCountByte...)
		s.RawBytes = append(s.RawBytes, b...)
	} else {
		s.pushOpCode(PUSHDATA4)
		s.RawBytes = append(s.RawBytes, trimmedCountByte...)
		s.RawBytes = append(s.RawBytes, b...)
	}
	return nil
}

func (s *ScriptBuilder) pushData(data interface{}) error {
	switch e := data.(type) {
	case UTXO:
		//reverse txID to little endian
		log.Printf("pusing %v %v\n", e.TXID, e.Index)
		b, err := hex.DecodeString(e.TXID)
		if err != nil {
			return err
		}
		littleEndianTXID := reverseBytes(b)
		index := e.Index
		s.RawBytes = append(s.RawBytes, littleEndianTXID...)
		s.pushInt(index)
		return nil
	case TradingVersion:
		s.RawBytes = append(s.RawBytes, byte(e))
		return nil
	case TransactionAttribute:
		s.RawBytes = append(s.RawBytes, byte(e))
		return nil
	case TransactionType:
		s.RawBytes = append(s.RawBytes, byte(e))
		return nil
	case NEOAddress:
		//when pushing neo address as an arg. we need length so we need to push a hex string
		return s.pushHexString(fmt.Sprintf("%x", e))
	case ScriptHash:
		s.RawBytes = append(s.RawBytes, e...)
		return nil
	case string:
		return s.pushHexString(e)
	case []byte:
		// length + data
		return s.pushHexString(hex.EncodeToString(e))
	case bool:
		if e == true {
			s.pushOpCode(PUSH1)
		} else {
			s.pushOpCode(PUSH0)
		}
		return nil
	case []interface{}:
		count := len(e)
		//reverse the array first
		for i := len(e) - 1; i >= 0; i-- {
			s.pushData(e[i])
		}
		s.pushData(count)
		s.pushOpCode(PACK)
		return nil
	case int:
		s.pushInt(e)
		return nil
	}
	return nil
}

func NewScriptHash(hexString string) (ScriptHash, error) {
	b, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	//we need to reverse the script hash to little endian
	reversed := reverseBytes(b)
	return ScriptHash(reversed), nil
}

// This is in a format of main(string operation, []object args) in c#
func (s *ScriptBuilder) generateContractInvocationScript(scriptHash ScriptHash, operation string, args []interface{}) []byte {
	if args != nil {
		s.pushData(args)
	}
	s.pushData([]byte(operation))                                     //operation is in string we need to convert it to hex first
	s.pushOpCode(APPCALL)                                             //use APPCALL only
	s.pushData(scriptHash)                                            // script hash of the smart contract that we want to invoke
	s.RawBytes = append([]byte{byte(len(s.RawBytes))}, s.RawBytes...) //the length of the entire raw bytes
	return s.ToBytes()
}

func (s *ScriptBuilder) generateTransactionAttributes(attributes map[TransactionAttribute][]byte) ([]byte, error) {

	count := len(attributes)
	s.pushLength(count) //number of transaction attributes
	// N x transaction attribute
	//transaction attribute =  TransactionAttribute + data.length + data
	for k, v := range attributes {
		s.pushData(k) //transaction attribute usage
		s.pushData(v) //push byte data in already includes the length of the data
	}

	return s.ToBytes(), nil
}

func (s *ScriptBuilder) generateTransactionInput(unspent Unspent, assetToSend NativeAsset, amountToSend float64) ([]byte, error) {
	//inputs = [input_count] + [[txID(32)] + [txIndex(2)]] = 34 x input_count bytes

	sendingAsset := unspent.Assets[assetToSend]
	if sendingAsset == nil {
		return nil, fmt.Errorf("Asset %v not found in UTXO", assetToSend)
	}

	if amountToSend > sendingAsset.TotalAmount() {
		return nil, fmt.Errorf("Don't have enough balance. Sending %v but only have %v", amountToSend, sendingAsset.TotalAmount())
	}

	//sort min first
	sendingAsset.SortMinFirst()

	runningAmount := float64(0)
	index := 0
	count := 0
	inputs := []UTXO{}
	for runningAmount < amountToSend {
		addingUTXO := sendingAsset.UTXOs[index]
		inputs = append(inputs, addingUTXO)
		runningAmount += addingUTXO.Value
		index += 1
		count += 1
	}

	s.pushLength(count)
	for _, v := range inputs {
		//push utxo data
		s.pushData(v)
	}

	return s.ToBytes(), nil
}

func (s *ScriptBuilder) generateTransactionOutput(assetToSend NativeAsset, amountToSend float64) ([]byte, error) {

	//output = [output_count] + [assetID(32)] + [amount(8)] + [sender_scripthash(20)] = 60 x output_count bytes
	//if the running
	return nil, nil
}
