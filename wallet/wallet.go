package wallet

import (
	"io/ioutil"
	"github.com/go-naivecoin/tx"
	"os"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"encoding/hex"
	"fmt"
	. "github.com/ahmetb/go-linq"
	"github.com/pkg/errors"
	"log"
)

const (
	PrivateKeyLocation = "./private_key"
)

func GetPrivateFromWallet() (string, error) {
	data, err := ioutil.ReadFile(PrivateKeyLocation)
	return hex.EncodeToString(data), err
}

func GetPublicFromWallet() (string, error) {
	privateKey, err := GetPrivateFromWallet()
	if err != nil {
		return "", err
	}
	return tx.GetPublicKey(privateKey)
}

func InitWallet() {
	if _, err := os.Stat(PrivateKeyLocation); os.IsExist(err) {
		return
	}

	privateKey, _ := secp256k1.GeneratePrivateKey()

	skKey := hex.EncodeToString(privateKey.Serialize())

	err := ioutil.WriteFile(PrivateKeyLocation, []byte(skKey), 0644)

	if err != nil {
		return
	}
}

func GetBalance(address string, unspentTxOuts tx.UnspentTxOuts) int64 {
	utxos := FindUnspentTxOuts(address, unspentTxOuts)
	return From(utxos).Select(func(i interface{}) interface{} {
		utxo := i.(tx.UnspentTxOut)
		return utxo.Amount
	}).SumInts()
}

func FindUnspentTxOuts(address string, unspentTxOuts tx.UnspentTxOuts) tx.UnspentTxOuts {
	var utxos tx.UnspentTxOuts
	From(unspentTxOuts).Where(func(i interface{}) bool {
		utxo := i.(tx.UnspentTxOut)
		return utxo.Address == address
	}).ToSlice(&utxos)
	return utxos
}

func FindTxOutsForAmount(amount int64, myUnspentTxOuts tx.UnspentTxOuts) (tx.UnspentTxOuts, int64, error) {
	currentAmount := int64(0)
	lastAmount := int64(0)
	var includedUnspentTxOuts tx.UnspentTxOuts
	From(myUnspentTxOuts).TakeWhile(func(i interface{}) bool {
		utxo := i.(tx.UnspentTxOut)
		lastAmount = currentAmount
		currentAmount += utxo.Amount
		return lastAmount < amount
	}).ToSlice(&includedUnspentTxOuts)

	if (lastAmount < amount) {
		lastAmount = currentAmount
	}

	if lastAmount < amount {
		msg := fmt.Sprintf("Cannot create transaction from the available unspent transaction outputs. Required amount: %d Avaliable unspentUtxos %v", amount, myUnspentTxOuts)
		return nil, 0, errors.New(msg)
	} else {
		return includedUnspentTxOuts,  lastAmount - amount, nil
	}
}

func CreateTxOuts(receiverAddress string, myAddress string, amount int64, leftOverAmount int64) []tx.TxOut {
	txOut1 := tx.TxOut{receiverAddress, amount}
	if leftOverAmount == 0 {
		return []tx.TxOut{txOut1}
	} else {
		leftOverTx := tx.TxOut{myAddress, leftOverAmount}
		return []tx.TxOut{txOut1, leftOverTx}
	}
}

func filterTxPoolTxs(unspentTxOuts tx.UnspentTxOuts, transactionPool tx.TransactionPool) tx.UnspentTxOuts {
	var txIns []tx.TxIn
	From(transactionPool).SelectMany(func(i interface{}) Query {
		t := i.(tx.Transaction)
		return From(t.TxIns)
	}).ToSlice(&txIns)

	var filteredUtxos tx.UnspentTxOuts

	From(unspentTxOuts).Where(func(i interface{}) bool {
		utxo := i.(tx.UnspentTxOut)

		found := From(txIns).FirstWith(func(j interface{}) bool {
			txIn := j.(tx.TxIn)
			return txIn.TxOutId == utxo.TxOutId && txIn.TxOutIndex == utxo.TxOutIndex
		})
		return found == nil
	}).ToSlice(&filteredUtxos)

	return filteredUtxos
}

func CreateTransaction(receiverAddress string, amount int64, privateKey string, unspentTxOuts tx.UnspentTxOuts, txPool tx.TransactionPool) (*tx.Transaction, error) {
	log.Printf("txPool: %v", txPool)

	myAddress, err := tx.GetPublicKey(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "CreateTransaction-GetPublicKey")
	}


	myUnspentTxOutsA := FindUnspentTxOuts(myAddress, unspentTxOuts)
	myUnspentTxOuts := filterTxPoolTxs(myUnspentTxOutsA, txPool)

	includedUnspentTxOuts, leftOverAmount, err := FindTxOutsForAmount(amount, myUnspentTxOuts)
	if err != nil {
		return nil,  errors.Wrap(err, "CreateTransaction-FindTxOutsForAmount")
	}

	var unsignedTxIns []tx.TxIn
	From(includedUnspentTxOuts).Select(func(i interface{}) interface{} {
		utxo := i.(tx.UnspentTxOut)
		return tx.TxIn{TxOutId: utxo.TxOutId, TxOutIndex: utxo.TxOutIndex}
	}).ToSlice(&unsignedTxIns)

	txOuts := CreateTxOuts(receiverAddress, myAddress, amount, leftOverAmount)

	transaction := tx.Transaction{
		TxIns:  unsignedTxIns,
		TxOuts: txOuts,
	}

	transaction.Id = transaction.GetTransactionId()

	var signedTxIns []tx.TxIn
	From(unsignedTxIns).SelectIndexed(func(index int, i interface{}) interface{} {
		txIn := i.(tx.TxIn)
		signature, err := transaction.SignTxIn(int64(index), privateKey, unspentTxOuts)
		if err != nil {
			panic(transaction)
		}
		txIn.Signature = signature
		return txIn
	}).ToSlice(&signedTxIns)

	transaction.TxIns = signedTxIns

	return &transaction, nil

}
