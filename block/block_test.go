package block_test

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"github.com/go-naivecoin/block"
	"github.com/go-naivecoin/tx"
	"fmt"
)

const (
	ADDRESS = "04115c42e757b2efb7671c578530ec191a1359381e6a71127a9d37c486fd30dae57e76dc58f693bd7e7010358ce6b165e483a2921010db67ac11b1b51b651953d2"
	//TX_ID   = ""
)

func TestNewBlock_IfHashIsEmpty_ThenCalculate(t *testing.T)  {
	newBlock := block.NewBlock(1, "", "91a73664bc84c0baa1fc75ea6e4aa6d1d20c5df664c724e3159aefc2e1186627",1465154715,nil, 0, 0)

	assert.Equal(t,"a5d76fec271299ff42eb51d3666f102c34ab9fc86ebb1dbb3edb64a393b1435b", newBlock.Hash)
}

func TestHasMatchesDifficulty(t *testing.T) {
	matched := block.HasMatchesDifficulty("0123456789abcdef", 7)

	assert.True(t, matched)
}

func TestHexToBin(t *testing.T)  {
	str := block.HexToBin("0123456789abcdef")

	assert.Equal(t, "0000000100100011010001010110011110001001101010111100110111101111",str)
}

func TestFindBlock(t *testing.T) {
	newBlock := block.FindBlock(1,"9cbfae34f219c6c217ea85a24e94b912a7ec1dc894248bab67fcb27497533a7e",1465154725, nil, 6)

	assert.Equal(t,int64(24),  newBlock.Nonce)
}

func TestSetUnpentTxOuts_TheGetUnpentTxOuts(t *testing.T) {
	var utxos tx.UnspentTxOuts = tx.UnspentTxOuts{tx.UnspentTxOut{"1", 1, ADDRESS, 1}}

	block.SetUnpentTxOuts(utxos)

	utxos2 := block.GetUnpentTxOuts()

	oldAddress := fmt.Sprintf("%p", &utxos)
	newAddress := fmt.Sprintf("%p", &utxos2)

	assert.NotEqual(t, oldAddress, newAddress)

}
