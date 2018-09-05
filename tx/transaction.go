package tx

import (
	. "github.com/ahmetb/go-linq"
	"fmt"
	"crypto/sha256"
	"encoding/json"
	"github.com/pkg/errors"
	"encoding/hex"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"regexp"
	"strings"
	"log"
)

type UnspentTxOut struct {
	TxOutId    string `json:"txOutId"`
	TxOutIndex int64  `json:"txOutIndex"`
	Address    string `json:"address"`
	Amount     int64  `json:"amount"`
}

type UnspentTxOuts []UnspentTxOut

func (utxos UnspentTxOuts) findUnspentTxOut(transactionId string, index int64) (UnspentTxOut, bool) {
	matched, ok := From(utxos).FirstWith(func(i interface{}) bool {
		utxo := i.(UnspentTxOut)
		return utxo.TxOutId == transactionId && utxo.TxOutIndex == index
	}).(UnspentTxOut)

	return matched, ok
}

type TxIn struct {
	TxOutId    string `json:"txOutId"`
	TxOutIndex int64  `json:"txOutIndex"`
	Signature  string `json:"signature"`
}

func (txIn *TxIn) validateTxIn(transaction *Transaction, aUnspentTxOuts UnspentTxOuts) bool {
	utxo, found := aUnspentTxOuts.findUnspentTxOut(txIn.TxOutId, txIn.TxOutIndex)
	if !found {
		bytes, _ := json.Marshal(txIn)
		log.Printf("referenced txOut not found: %s", string(bytes[:]))
		return false
	}

	pubKeyBytes, err := hex.DecodeString(utxo.Address)
	if err != nil {
		log.Printf("%s", err.Error())
		return false
	}

	pubKey, err := secp256k1.ParsePubKey(pubKeyBytes)
	if err != nil {
		log.Printf("%s", err.Error())
		return false
	}

	sigBytes, err := hex.DecodeString(txIn.Signature)
	if err != nil {
		log.Printf("%s", err.Error())
		return false
	}

	signature, err := secp256k1.ParseDERSignature(sigBytes, pubKey.Curve)
	if err != nil {
		log.Printf("%s", err.Error())
		return false
	}

	return signature.Verify([]byte(transaction.Id), pubKey)
}
func (txIn *TxIn) getTxInAmount(aUnspentTxOuts UnspentTxOuts) int64 {
	utxo, found := aUnspentTxOuts.findUnspentTxOut(txIn.TxOutId, txIn.TxOutIndex)
	if !found {
		return 0
	}

	return utxo.Amount
}

type TxOut struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
}

type Transaction struct {
	Id     string  `json:"id"`
	TxIns  []TxIn  `json:"txIns"`
	TxOuts []TxOut `json:"txOuts"`
}

const COINBASE_AMOUNT int64 = 50

// todo: this may need refine
func (t *Transaction) GetTransactionId() string {
	txInContent := From(t.TxIns).Select(func(i interface{}) interface{} {
		txIn := i.(TxIn)
		return fmt.Sprintf("%s%d", txIn.TxOutId, txIn.TxOutIndex)
	}).AggregateWithSeed("", func(i interface{}, i2 interface{}) interface{} {
		iStr := i.(string)
		i2Str := i2.(string)
		return iStr + i2Str
	}).(string)

	txOutContent := From(t.TxOuts).Select(func(i interface{}) interface{} {
		txOut := i.(TxOut)
		return fmt.Sprintf("%s%d", txOut.Address, txOut.Amount)
	}).AggregateWithSeed("", func(i interface{}, i2 interface{}) interface{} {
		iStr := i.(string)
		i2Str := i2.(string)
		return iStr + i2Str
	}).(string)

	hashStr := fmt.Sprintf("%s%s", txInContent, txOutContent)
	bytes := sha256.Sum256([]byte(hashStr))
	return fmt.Sprintf("%x", bytes)
}

func (t *Transaction) ValidateTransaction(aUnspentTxOuts UnspentTxOuts) bool {
	if t.GetTransactionId() != t.Id {
		log.Printf("Invalid tx id： %s", t.Id)
	}

	hasValidTxIns := From(t.TxIns).Select(func(i interface{}) interface{} {
		txIn := i.(TxIn)
		return txIn.validateTxIn(t, aUnspentTxOuts)
	}).AggregateWithSeed(true, func(i interface{}, i2 interface{}) interface{} {
		validate1 := i.(bool)
		validate2 := i2.(bool)
		return validate1 && validate2
	}).(bool)

	if !hasValidTxIns {
		log.Printf("some of the txIns are invalid in tx: %s", t.Id)
		return false
	}

	totalTxInValues := From(t.TxIns).Select(func(i interface{}) interface{} {
		txIn := i.(TxIn)
		return txIn.getTxInAmount(aUnspentTxOuts)
	}).AggregateWithSeed(int64(0), func(i interface{}, i2 interface{}) interface{} {
		amount1 := i.(int64)
		amount2 := i2.(int64)
		return amount1 + amount2
	}).(int64)

	totalTxOutValues := From(t.TxOuts).Select(func(i interface{}) interface{} {
		txOut := i.(TxOut)
		return txOut.Amount
	}).AggregateWithSeed(int64(0), func(i interface{}, i2 interface{}) interface{} {
		amount1 := i.(int64)
		amount2 := i2.(int64)
		return amount1 + amount2
	}).(int64)

	if totalTxInValues != totalTxOutValues {
		log.Printf("totalTxInValues != totalTxOutValues in tx: %s", t.Id)
		return false
	}

	return true
}

func (t *Transaction) validateCoinbaseTx(blockIndex int64) bool {
	if t.GetTransactionId() != t.Id {
		log.Printf("invalid coinbase tx id: %s", t.Id)
		return false
	}

	if len(t.TxIns) != 1 {
		log.Printf("one txIn must be specified in the coinbase transaction")
		return false
	}

	if t.TxIns[0].TxOutIndex != blockIndex {
		log.Printf("the txIn Signature in coinbase tx must be the block height")
		return false
	}

	if len(t.TxOuts) != 1 {
		log.Printf("invalid number of txOuts in coinbase transaction")
		return false
	}

	if t.TxOuts[0].Amount != COINBASE_AMOUNT {
		log.Printf("invalid coinbase amount in coinbase transaction")
		return false
	}

	return true
}

func (t *Transaction) SignTxIn(txInIndex int64, privateKey string, aUnspentTxOuts UnspentTxOuts) (string, error) {

	txIn := t.TxIns[txInIndex]
	dataToSign := []byte(t.Id)

	utxo, found := aUnspentTxOuts.findUnspentTxOut(txIn.TxOutId, txIn.TxOutIndex)
	if !found {
		log.Printf("could not find referenced txOut")
		panic(t)
	}

	skBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		log.Printf(err.Error())
		return "", errors.Wrap(err, "SignTxIn- hex.DecodeString")
	}

	privKey, pubKey := secp256k1.PrivKeyFromBytes(skBytes)

	pbBytes := pubKey.SerializeUncompressed()

	publicKey := hex.EncodeToString(pbBytes)

	if publicKey != utxo.Address {
		return "", errors.New("trying to sign an input with private key that does not match the address that is referenced in txIn")
	}

	signature, err := privKey.Sign(dataToSign)

	if err != nil {
		log.Printf("%s" ,err.Error())
		return "", errors.Wrap(err, "SignTxIn- privKey.Sign")
	}

	return hex.EncodeToString(signature.Serialize()), nil
}

func GetPublicKey(privateKey string) (string, error) {
	pkBytes, err := hex.DecodeString(privateKey)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	_, pubKey := secp256k1.PrivKeyFromBytes(pkBytes)

	return hex.EncodeToString(pubKey.SerializeUncompressed()), nil
}

func validateBlockTransactions(aTransactions []Transaction, aUnspentTxOuts UnspentTxOuts, blockIndex int64) bool {
	coinbaseTx := aTransactions[0]

	if !coinbaseTx.validateCoinbaseTx(blockIndex) {
		bytes, _ := json.Marshal(coinbaseTx)
		log.Printf("invalid coinbase transaction: %s", string(bytes[:]))
		return false
	}

	var txIns []TxIn
	From(aTransactions).SelectMany(func(i interface{}) Query {
		t := i.(Transaction)
		return From(t.TxIns)
	}).ToSlice(&txIns)

	distinctedLen := From(txIns).DistinctBy(func(i interface{}) interface{} {
		txIn := i.(TxIn)
		return fmt.Sprintf("%s%d", txIn.TxOutId, txIn.TxOutIndex)
	}).Count()

	if distinctedLen != len(txIns) {
		log.Printf("Has duplicated transaction")
		return false
	}

	normalTransactions := aTransactions[1:]

	isValid := From(normalTransactions).Select(func(i interface{}) interface{} {
		t := i.(Transaction)
		return t.ValidateTransaction(aUnspentTxOuts)
	}).AggregateWithSeed(true, func(i interface{}, i2 interface{}) interface{} {
		b := i.(bool)
		b2 := i2.(bool)
		return b && b2
	}).(bool)

	return isValid
}

func ProcessTransactions(newTransactions []Transaction, aUnspentTxOuts UnspentTxOuts, blockIndex int64) (UnspentTxOuts, error) {
	if !validateBlockTransactions(newTransactions, aUnspentTxOuts, blockIndex) {
		log.Printf("invalid block transactions")
		return nil, errors.New("invalid block transactions")
	}

	newUnspentTxOutsQuery := From(newTransactions).SelectMany(func(i interface{}) Query {
		transaction := i.(Transaction)
		return From(transaction.TxOuts).SelectIndexed(func(index int, txOutI interface{}) interface{} {
			txOut := txOutI.(TxOut)
			utxo := UnspentTxOut{
				TxOutId:    transaction.Id,
				TxOutIndex: int64(index),
				Address:    txOut.Address,
				Amount:     txOut.Amount,
			}
			return utxo
		})
	})

	var resultingUnspentTxOuts UnspentTxOuts

	var consumedTxOuts UnspentTxOuts

	From(newTransactions).SelectMany(func(t interface{}) Query {
		transaction := t.(Transaction)
		return From(transaction.TxIns).Select(func(txinI interface{}) interface{} {
			txIn := txinI.(TxIn)
			utxo := UnspentTxOut{
				TxOutId:    txIn.TxOutId,
				TxOutIndex: txIn.TxOutIndex,
				Address:    "",
				Amount:     0,
			}
			return utxo
		})
	}).ToSlice(&consumedTxOuts)

	From(aUnspentTxOuts).Where(func(i interface{}) bool {
		utxo := i.(UnspentTxOut)
		_, found := consumedTxOuts.findUnspentTxOut(utxo.TxOutId, utxo.TxOutIndex)
		return !found
	}).Concat(newUnspentTxOutsQuery).ToSlice(&resultingUnspentTxOuts)

	return resultingUnspentTxOuts, nil
}

func GetCoinbaseTransaction(address string, blockIndex int64) Transaction {
	var txIn TxIn = TxIn{TxOutIndex: blockIndex}
	var txOut TxOut = TxOut{Address: address, Amount: COINBASE_AMOUNT}
	var transaction Transaction = Transaction{
		"",
		[]TxIn{txIn},
		[]TxOut{txOut},
	}

	transaction.Id = transaction.GetTransactionId()

	return transaction
}

func IsValidAddress(address string) bool {
	if len(address) != 130 {
		log.Printf("invalid public key length")
		return false
	} else if matched, _ := regexp.MatchString("^[a-fA-F0-9]+$", address); !matched {
		log.Printf("public key must contain only hex characters'")
		return false
	} else if !strings.HasPrefix(address, "04") {
		log.Printf("public key must start with 04'")
		return false
	}
	return true
}
