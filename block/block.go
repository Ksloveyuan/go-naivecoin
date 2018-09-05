package block

import (
	"crypto/sha256"
	"time"
	"fmt"
	. "github.com/ahmetb/go-linq"
	"strconv"
	"strings"
	"math"
	"github.com/go-naivecoin/tx"
	"encoding/json"
	"bytes"
	"github.com/go-naivecoin/wallet"
	"github.com/pkg/errors"
	"encoding/gob"
	"log"
)

type Block struct {
	Index        int64             `json:"index"`
	Hash         string            `json:"hash"`
	PreviousHash string            `json:"previousHash"`
	Timestamp    int64             `json:"timestamp"`
	Data         [] tx.Transaction `json:"data"`
	Difficulty   int               `json:"difficulty"`
	Nonce        int64             `json:"nonce"`
}

const (
	BLOCK_GENERATION_INTERVAL      int = 10
	DIFFICULTY_ADJUSTMENT_INTERVAL int = 10
)

func NewBlock(index int64, hash string, previousHash string, timestamp int64, data []tx.Transaction, difficulty int, nonce int64) Block {
	block := Block{index, hash, previousHash, timestamp, data, difficulty, nonce}

	if block.Hash == "" {
		block.Hash = block.calculateHashForBlock()
	}

	return block
}

var genesisTransaction = tx.Transaction{
	TxIns:  []tx.TxIn{tx.TxIn{Signature: "", TxOutId: "", TxOutIndex: 0}},
	TxOuts: []tx.TxOut{{Address: "04bfcab8722991ae774db48f934ca79cfb7dd991229153b9f732ba5334aafcd8e7266e47076996b55a14bf9913ee3145ce0cfc1372ada8ada74bd287450313534a", Amount: 50}},
	Id:     "e655f6a5f26dc9b4cac6e46f52336428287759cf81ef5ff10854f69d68f43fa3",
}

var genesisBlock = NewBlock(0, "91a73664bc84c0baa1fc75ea6e4aa6d1d20c5df664c724e3159aefc2e1186627", "", 1465154705, []tx.Transaction{genesisTransaction}, 1, 0)

var blockchain = []Block{genesisBlock}

var emptyUtxos = make([]tx.UnspentTxOut, 0)
var unspentTxOuts, _ = tx.ProcessTransactions(blockchain[0].Data, emptyUtxos, 0)

func GetUnpentTxOuts() tx.UnspentTxOuts {
	var utxos tx.UnspentTxOuts
	var buff bytes.Buffer
	gob.NewEncoder(&buff).Encode(unspentTxOuts)
	gob.NewDecoder(bytes.NewBuffer(buff.Bytes())).Decode(&utxos)
	return utxos
}

func SetUnpentTxOuts(newUtxos tx.UnspentTxOuts) {
	log.Printf("replacing unspentTxouts with: %v", newUtxos)
	unspentTxOuts = newUtxos
}

func GetBlockchain() []Block {
	return blockchain
}

func GetLatestBlock() Block {
	return From(blockchain).Last().(Block)
}

func getDifficulty(aBlockchain []Block) int {
	latestBlock := GetLatestBlock()
	if latestBlock.Index%(int64(DIFFICULTY_ADJUSTMENT_INTERVAL)) == 0 && latestBlock.Index != 0 {
		return getAdjustedDifficulty(latestBlock, aBlockchain)
	} else {
		return latestBlock.Difficulty
	}
}
func getAdjustedDifficulty(latestBlock Block, aBlockchain []Block) int {
	prevAdjustmentBlock := blockchain[len(blockchain)-DIFFICULTY_ADJUSTMENT_INTERVAL]
	timeExpected := int64(BLOCK_GENERATION_INTERVAL * DIFFICULTY_ADJUSTMENT_INTERVAL)
	timeTaken := latestBlock.Timestamp - prevAdjustmentBlock.Timestamp

	if timeTaken < timeExpected/2 {
		return prevAdjustmentBlock.Difficulty + 1
	} else if timeTaken > timeExpected*2 {
		return prevAdjustmentBlock.Difficulty - 1
	} else {
		return prevAdjustmentBlock.Difficulty
	}
}

func GenerateRawBlock(data []tx.Transaction) *Block {
	previousBlock := GetLatestBlock()
	difficulty := getDifficulty(blockchain)
	nextIndex := previousBlock.Index + 1
	nextTimestamp := time.Now().Unix()
	newBlock := FindBlock(nextIndex, previousBlock.Hash, nextTimestamp, data, difficulty)

	if AddBlockToChain(newBlock) {
		return &newBlock
	} else {
		return nil
	}
}

func GetMyUnspentTransactionOutputs() tx.UnspentTxOuts {
	address, err := wallet.GetPublicFromWallet()
	if err != nil {
		fmt.Errorf("%s", err.Error())
		panic(err)
	}

	return wallet.FindUnspentTxOuts(address, GetUnpentTxOuts())
}

func GenerateNextBlock() *Block {
	address, err := wallet.GetPublicFromWallet()
	if err != nil {
		fmt.Errorf("%s", err.Error())
		panic(err)
	}

	coinbaseTx := tx.GetCoinbaseTransaction(address, GetLatestBlock().Index+1)
	var blockData = []tx.Transaction{coinbaseTx}
	txPool := tx.GetTransactionPool()
	blockData = append(blockData, txPool...)

	return GenerateRawBlock(blockData)
}

func GenerateNextBlockWithTransation(receiverAddress string, amount int64) (*Block, error) {
	if !tx.IsValidAddress(receiverAddress) {
		return nil, errors.New("Invalid address")
	}

	coinbaseTx := tx.GetCoinbaseTransaction(receiverAddress, GetLatestBlock().Index+1)
	privateKey, err := wallet.GetPrivateFromWallet()
	if err != nil {
		return nil, err
	}

	transaction, err := wallet.CreateTransaction(receiverAddress, amount, privateKey, GetUnpentTxOuts(), tx.GetTransactionPool())
	if err != nil {
		return nil, err
	}

	blockData := []tx.Transaction{coinbaseTx, *transaction}

	return GenerateRawBlock(blockData), nil
}

func FindBlock(index int64, previousHash string, timestamp int64, data []tx.Transaction, difficulty int) Block {
	var nonce int64 = 0
	for {
		hash := calculateHash(index, previousHash, timestamp, data, difficulty, nonce)
		if HasMatchesDifficulty(hash, difficulty) {
			return NewBlock(index, hash, previousHash, timestamp, data, difficulty, nonce)
		}
		nonce++
	}
}

func GetAccountBalance() (int64, error) {
	publicKey, err := wallet.GetPublicFromWallet()
	if err != nil {
		return 0, err
	}

	return wallet.GetBalance(publicKey, GetUnpentTxOuts()), nil
}

func SendTransaction(address string, amount int64) (*tx.Transaction, error) {
	privateKey, err := wallet.GetPrivateFromWallet()
	if err != nil {
		return nil, err
	}

	transaction, err := wallet.CreateTransaction(address, amount, privateKey, GetUnpentTxOuts(), tx.GetTransactionPool())
	if err != nil {
		return nil, err
	}

	_, err = tx.AddToTransactionPool(transaction, GetUnpentTxOuts())
	if err != nil {
		log.Print(err.Error())
	}

	return transaction, nil
}

func HasMatchesDifficulty(hash string, difficulty int) bool {
	hexStr := HexToBin(hash)
	difficultyPrefix := strings.Repeat("0", difficulty)
	matched := strings.HasPrefix(hexStr, difficultyPrefix)
	return matched
}

func HexToBin(s string) (binString string) {
	characters := strings.Split(s, "")
	for _, c := range characters {
		i, _ := strconv.ParseUint(c, 16, 4)
		binString = fmt.Sprintf("%s%04b", binString, i)
	}
	return
}

func calculateHash(index int64, previousHash string, timestamp int64, data []tx.Transaction, difficulty int, nonce int64) string {
	idString := From(data).Select(func(i interface{}) interface{} {
		tx := i.(tx.Transaction)
		return tx.Id
	}).OrderBy(func(i interface{}) interface{} {
		str := i.(string)
		return str
	}).AggregateWithSeed("", func(i interface{}, i2 interface{}) interface{} {
		str1 := i.(string)
		str2 := i2.(string)
		return str1 + str2
	}).(string)

	hashStr := fmt.Sprintf("%d%s%d%s%d%d", index, previousHash, timestamp, idString, difficulty, nonce)
	bytes := sha256.Sum256([]byte(hashStr))
	return fmt.Sprintf("%x", bytes)
}

func AddBlockToChain(newBlock Block) bool {
	if isValidNewBlock(newBlock, GetLatestBlock()) {
		retVal, err := tx.ProcessTransactions(newBlock.Data, GetUnpentTxOuts(), newBlock.Index)
		if err != nil {
			return false
		} else {
			blockchain = append(blockchain, newBlock)
			SetUnpentTxOuts(retVal)
			tx.UpdateTransactionPool(retVal)
			return true
		}
	}

	return false
}

func isValidNewBlock(newBlock Block, previousBlock Block) bool {
	if previousBlock.Index+1 != newBlock.Index {
		fmt.Errorf("invalid Index")
		return false
	} else if previousBlock.Hash != newBlock.PreviousHash {
		fmt.Errorf("invalid previous Hash")
		return false
	} else if newBlock.calculateHashForBlock() != newBlock.Hash {
		fmt.Errorf("invalid Hash")
		return false
	} else if !isValidTimestamp(newBlock, previousBlock) {
		fmt.Errorf("invalid timestamp")
		return false
	} else if !hasValidHash(newBlock) {
		return false
	}

	return true
}

func getAccumulatedDifficulty(aBlockchain []Block) float64 {
	return From(aBlockchain).Select(func(i interface{}) interface{} {
		block := i.(Block)
		value := math.Pow(2, float64(block.Difficulty))
		return value
	}).AggregateWithSeed(float64(0), func(i interface{}, j interface{}) interface{} {
		nI := i.(float64)
		nJ := j.(float64)
		return nI + nJ
	}).(float64)
}

func hasValidHash(block Block) bool {
	if !hasMatchesBlockContent(block) {
		log.Printf("invalid hash, got %s", block.Hash)
		return false
	}

	if !HasMatchesDifficulty(block.Hash, block.Difficulty) {
		log.Printf("block difficulty not satisfied. Expected: %d got: %s", block.Difficulty, block.Hash)
		return false
	}

	return true
}

func hasMatchesBlockContent(block Block) bool {
	blockHash := block.calculateHashForBlock()
	return blockHash == block.Hash
}

func isValidTimestamp(newBlock Block, previousBlock Block) bool {
	currentTimestamp := time.Now().Unix()
	const gap int64 = 60
	return (previousBlock.Timestamp-gap) < newBlock.Timestamp && (newBlock.Timestamp-gap) < currentTimestamp;
}

func isValidChain(blockchainToValidate []Block) tx.UnspentTxOuts {
	toValidateBytes, err := json.Marshal(blockchainToValidate[0])
	if err != nil {
		log.Printf("can't marshal %v", blockchainToValidate[0])
		return nil
	}

	basedBytes, _ := json.Marshal(genesisBlock)
	if bytes.Compare(toValidateBytes, basedBytes) != 0 {
		return nil
	}

	var aUnspentTxOuts tx.UnspentTxOuts

	for i := 0; i < len(blockchainToValidate); i++ {
		currentBlock := blockchainToValidate[i]
		if i != 0 && !isValidNewBlock(blockchainToValidate[i], blockchainToValidate[i-1]) {
			return nil
		}

		aUnspentTxOuts, err = tx.ProcessTransactions(currentBlock.Data, aUnspentTxOuts, currentBlock.Index)
		if err != nil {
			log.Printf("Invalid transactions")
			return nil
		}
	}

	return aUnspentTxOuts
}

func ReplaceChain(newBlocks []Block) {
	aUnspentTxOuts := isValidChain(newBlocks)
	validChain := aUnspentTxOuts != nil

	if validChain && getAccumulatedDifficulty(newBlocks) > getAccumulatedDifficulty(GetBlockchain()) {
		log.Printf("Received blockchain is valid. Replacing current blockchain with received blockchain")
		blockchain = newBlocks
		SetUnpentTxOuts(aUnspentTxOuts)
		tx.UpdateTransactionPool(unspentTxOuts)
	} else {
		log.Printf("Received blockchain invalid")
	}
}

func HandleReceivedTransaction(transaction *tx.Transaction) {
	tx.AddToTransactionPool(transaction, GetUnpentTxOuts())
}

func (block *Block) calculateHashForBlock() string {
	return calculateHash(block.Index, block.PreviousHash, block.Timestamp, block.Data, block.Difficulty, block.Nonce)
}
