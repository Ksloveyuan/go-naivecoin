package tx

import (
	"encoding/gob"
	"bytes"
	"github.com/pkg/errors"
	"log"
	. "github.com/ahmetb/go-linq"
)

type TransactionPool []Transaction

var transactionPool TransactionPool = make(TransactionPool, 0)

func GetTransactionPool() TransactionPool {
	var theTranactionPool TransactionPool

	var buff bytes.Buffer
	gob.NewEncoder(&buff).Encode(transactionPool)
	gob.NewDecoder(bytes.NewBuffer(buff.Bytes())).Decode(&theTranactionPool)

	return theTranactionPool
}

func AddToTransactionPool(tx *Transaction, unspentTxOuts UnspentTxOuts) (bool, error) {
	if !tx.ValidateTransaction(unspentTxOuts) {
		return false, errors.New("Trying to add invalid tx to pool")
	}

	if !IsValidTxForPool(tx, transactionPool) {
		return false, errors.New("Trying to add invalid tx to pool")
	}

	transactionPool = append(transactionPool, *tx)

	return true, nil
}

func UpdateTransactionPool(unspentTxOuts UnspentTxOuts) {
	var invalidTxs []Transaction
	From(transactionPool).Where(func(i interface{}) bool {
		tx := i.(Transaction)
		_, foundInvalidTX := From(tx.TxIns).FirstWith(func(j interface{}) bool {
			txIn := j.(TxIn)
			_, foundUTXO := unspentTxOuts.findUnspentTxOut(txIn.TxOutId, txIn.TxOutIndex)
			return !foundUTXO
		}).(TxIn)

		return foundInvalidTX
	}).ToSlice(&invalidTxs)

	if len(invalidTxs) > 0 {
		var updatedTransactionPool []Transaction

		log.Printf("removing the following transactions from txPool: %v", invalidTxs)

		From(transactionPool).ExceptBy(From(invalidTxs), func(i interface{}) interface{} {
			tx := i.(Transaction)
			return tx.Id
		}).ToSlice(&updatedTransactionPool)

		transactionPool = updatedTransactionPool
	}
}

func (t TransactionPool) GetTxPoolIns() []TxIn {
	var txIns []TxIn

	From(transactionPool).SelectMany(func(i interface{}) Query {
		tx := i.(Transaction)
		return From(tx.TxIns)
	}).ToSlice(&txIns)

	return txIns
}

func IsValidTxForPool(transaction *Transaction, pools TransactionPool) bool {
	txPoolIns := pools.GetTxPoolIns()

	duplicatedTx := From(transaction.TxIns).FirstWith(func(i interface{}) bool {
		txIn := i.(TxIn)

		found := From(txPoolIns).FirstWith(func(j interface{}) bool {
			txPoolIn := j.(TxIn)
			return txIn.TxOutIndex == txPoolIn.TxOutIndex && txIn.TxOutId == txPoolIn.TxOutId
		})

		return found != nil
	})

	return duplicatedTx == nil
}
