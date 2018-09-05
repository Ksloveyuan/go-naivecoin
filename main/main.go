package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-naivecoin/block"
	"net/http"
	"log"
	"github.com/go-naivecoin/p2p"
	. "github.com/ahmetb/go-linq"
	"golang.org/x/net/websocket"
	"github.com/go-naivecoin/tx"
	"github.com/go-naivecoin/wallet"
)

type BlockRequest struct {
	Transactions []tx.Transaction `json:"transactions"`
}

type PeerRequest struct {
	Url string `json`
}

type TransactionRequest struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount"`
}

func main() {
	r := gin.Default()

	r.GET("/blocks", func(c *gin.Context) {
		result := block.GetBlockchain()
		c.JSON(http.StatusOK, result)
	})

	r.GET("/block/:hash", func(c *gin.Context) {
		hash := c.Param("hash")

		//hash := "91a73664bc84c0baa1fc75ea6e4aa6d1d20c5df664c724e3159aefc2e1186627"

		blockChain := block.GetBlockchain()

		block, found := From(blockChain).FirstWith(func(i interface{}) bool {
			block := i.(block.Block)
			return block.Hash == hash
		}).(block.Block)

		if found {
			c.JSON(http.StatusOK, block)
		} else {
			c.JSON(http.StatusOK, gin.H{
				"block": "Not found",
			})
		}
	})

	r.GET("/transaction/:id", func(c *gin.Context) {
		id := c.Param("id")

		blockChain := block.GetBlockchain()

		transaction, found := From(blockChain).SelectMany(func(i interface{}) Query {
			block := i.(block.Block)
			return From(block.Data)
		}).FirstWith(func(i interface{}) bool {
			tx := i.(tx.Transaction)
			return tx.Id == id
		}).(tx.Transaction)

		if found {
			c.JSON(http.StatusOK, transaction)
		} else {
			c.JSON(http.StatusOK, gin.H{
				"transaction": "Not found",
			})
		}
	})

	r.GET("/address/:address", func(c *gin.Context) {
		address := c.Param("address")

		utxos := block.GetUnpentTxOuts()

		var referencedUtxos tx.UnspentTxOuts

		From(utxos).Where(func(i interface{}) bool {
			utxo := i.(tx.UnspentTxOut)
			return utxo.Address == address
		}).ToSlice(&referencedUtxos)

		c.JSON(http.StatusOK, gin.H{
			"unspentTxOuts": referencedUtxos,
		})
	})

	r.GET("/unspentTransactionOutputs", func(c *gin.Context) {
		utxos := block.GetUnpentTxOuts()

		c.JSON(http.StatusOK, utxos)
	})

	r.GET("/myUnspentTransactionOutputs", func(c *gin.Context) {
		myUtxos := block.GetMyUnspentTransactionOutputs()
		c.JSON(http.StatusOK, myUtxos)
	})

	r.POST("/mineRawBlock", func(c *gin.Context) {
		var blockRequest BlockRequest

		if err := c.ShouldBindJSON(&blockRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		nextBlock := block.GenerateRawBlock(blockRequest.Transactions)
		p2p.BroadcastLatest()

		if nextBlock == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "could not generate block",
			})
		} else {
			c.JSON(http.StatusOK, *nextBlock)
		}
	})

	r.POST("/mineBlock", func(c *gin.Context) {
		nextBlock := block.GenerateNextBlock()
		p2p.BroadcastLatest()

		if nextBlock == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "could not generate block",
			})
		} else {
			c.JSON(http.StatusOK, *nextBlock)
		}
	})

	r.GET("/balance", func(c *gin.Context) {
		balance, err := block.GetAccountBalance()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"balance": balance,
			})
		}
	})

	r.GET("/address", func(c *gin.Context) {
		address, err := wallet.GetPublicFromWallet()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"address": address,
			})
		}
	})

	r.POST("/mineTransactions", func(c *gin.Context) {
		var transactionRequest TransactionRequest

		if err := c.ShouldBindJSON(&transactionRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		block, err := block.GenerateNextBlockWithTransation(transactionRequest.Address, transactionRequest.Amount)

		if err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
		} else {
			c.JSON(http.StatusOK, *block)
		}
	})

	r.POST("/sendTransaction", func(c *gin.Context) {
		var transactionRequest TransactionRequest

		if err := c.ShouldBindJSON(&transactionRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		transaction, err := block.SendTransaction(transactionRequest.Address, transactionRequest.Amount)

		if err != nil {
			p2p.BroadCastTransactionPool()
			c.JSON(http.StatusBadRequest, err.Error())
		} else {
			c.JSON(http.StatusOK, *transaction)
		}
	})

	r.GET("/transactionPool", func(c *gin.Context) {
		txPool := tx.GetTransactionPool()
		c.JSON(http.StatusOK, txPool)
	})

	r.GET("/peers", func(c *gin.Context) {

		sockets := p2p.GetSockets()

		var urls []string

		From(sockets).Select(func(i interface{}) interface{} {
			socket := i.(*websocket.Conn)
			return socket.Request().URL.String()
		}).ToSlice(&urls)

		c.JSON(http.StatusOK, gin.H{
			"data": urls,
		})
	})

	r.POST("/addPeer", func(c *gin.Context) {
		var peerRequest PeerRequest

		if err := c.ShouldBindJSON(&peerRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		p2p.ConnectToPeer(peerRequest.Url)

		c.JSON(http.StatusOK, gin.H{
			"data": "Success",
		})
	})

	r.GET("/", func(c *gin.Context) {
		conn, err := p2p.Wsupgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Failed to set websocket upgrade: %+v", err)
			return
		}

		p2p.InitConnection(conn)

		c.JSON(http.StatusOK, gin.H{
			"data": "Success",
		})
	})

	wallet.InitWallet()
	r.Run() // listen and serve on 0.0.0.0:8080
}
