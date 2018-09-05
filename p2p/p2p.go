package p2p

import (
	"github.com/gorilla/websocket"
	. "github.com/ahmetb/go-linq"
	"github.com/go-naivecoin/block"
	"log"
	"net/url"
	"encoding/json"
	"time"
	"github.com/go-naivecoin/tx"
)

var sockets [] *websocket.Conn

type MessageType int

const (
	QUERY_LATEST              MessageType = 0
	QUERY_ALL                 MessageType = 1
	RESPONSE_BLOCKCHAIN       MessageType = 2
	QUERY_TRANSACTION_POOL    MessageType = 3
	RESPONSE_TRANSACTION_POOL MessageType = 4
)

type P2PMessage struct {
	Type MessageType `json:"type"`
	Data string      `json:"data"`
}

var Wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func InitConnection(conn *websocket.Conn) {

	sockets = append(sockets, conn)

	conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("Close connection, %s", text)
		closeConnection(conn)
		return nil
	})

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Failed connection")
				closeConnection(conn)
				break
			}

			var message P2PMessage
			if err := json.Unmarshal(msg, &message); err != nil {
				log.Fatal("unmarshal", err)
			}

			log.Printf("Received message: %v", message)

			switch message.Type {
			case QUERY_LATEST:
				write(responseLatestMsg(), conn)
				break
			case QUERY_ALL:
				write(responseChainMsg(), conn)
				break
			case RESPONSE_BLOCKCHAIN:
				var receivedBlocks []block.Block
				bytes := []byte(message.Data)
				if err := json.Unmarshal(bytes, &receivedBlocks); err != nil {
					log.Printf("invalid blocks receive, %s", message.Data)
					log.Printf("unmarshal error: %s", err.Error())
					break
				}

				handleReceivedResponse(receivedBlocks)
				break
			case QUERY_TRANSACTION_POOL:
				write(responseTransactionPoolMsg(), conn)
				break
			case RESPONSE_TRANSACTION_POOL:
				var receivedTransactions tx.TransactionPool
				bytes := []byte(message.Data)
				if err := json.Unmarshal(bytes, &receivedTransactions); err != nil {
					log.Printf("invalid transaction receive, %s", message.Data)
					log.Printf("unmarshal error: %s", err.Error())
					break
				}

				for _, transaction := range receivedTransactions {
					block.HandleReceivedTransaction(&transaction)
					BroadCastTransactionPool()
				}

				break
			}
		}
	}()

	write(queryChainLengthMsg(), conn)
	time.AfterFunc(500*time.Millisecond, func() {
		write(queryTransactionPoolMsg(), conn)
	})

}

func closeConnection(s *websocket.Conn) {
	var newSockets []*websocket.Conn
	From(sockets).Where(func(i interface{}) bool {
		socket := i.(*websocket.Conn)
		return socket != s
	}).ToSlice(&newSockets)

	s.Close()

	sockets = newSockets
}

func Broadcast(message P2PMessage) {
	From(sockets).ForEach(func(i interface{}) {
		socket := i.(*websocket.Conn)
		write(message, socket)
	})
}

func write(message P2PMessage, conn *websocket.Conn) {
	if bytes, err := json.Marshal(message); err != nil {
		log.Printf("encode error: %s", err.Error())
	} else {
		conn.WriteMessage(websocket.TextMessage, bytes)
	}
}

func queryChainLengthMsg() P2PMessage {
	return P2PMessage{Type: QUERY_LATEST, Data: ""}
}

func queryAllMsg() P2PMessage {
	return P2PMessage{Type: QUERY_ALL, Data: ""}
}

func queryTransactionPoolMsg() P2PMessage {
	return P2PMessage{Type: QUERY_TRANSACTION_POOL, Data: ""}
}

func responseTransactionPoolMsg() P2PMessage {
	bytes, _ := json.Marshal(tx.GetTransactionPool())
	jsonStr := string(bytes[:])
	return P2PMessage{Type: RESPONSE_TRANSACTION_POOL, Data: jsonStr}
}

func responseChainMsg() P2PMessage {
	bytes, err := json.Marshal(block.GetBlockchain())
	if err != nil {
		log.Fatal("encode error:", err)

	}
	jsonString := string(bytes[:])
	return P2PMessage{Type: RESPONSE_BLOCKCHAIN, Data: jsonString}
}

func responseLatestMsg() P2PMessage {
	blocks := []block.Block{block.GetLatestBlock()}
	bytes, err := json.Marshal(blocks)
	if err != nil {
		log.Fatal("encode error:", err)
	}
	return P2PMessage{Type: RESPONSE_BLOCKCHAIN, Data: string(bytes)}
}

func handleReceivedResponse(receivedBlocks []block.Block) {
	if len(receivedBlocks) == 0 {
		log.Print("received block chain size of 0")
		return
	}

	latestReceivedBlock := From(receivedBlocks).Last().(block.Block)
	latestBlockHeld := block.GetLatestBlock()

	if latestReceivedBlock.Index > latestBlockHeld.Index {
		log.Printf("blockchian possibly behind. We got: %d Peer got: %d", latestBlockHeld.Index, latestReceivedBlock.Index)

		if latestBlockHeld.Hash == latestReceivedBlock.PreviousHash {
			if block.AddBlockToChain(latestReceivedBlock) {
				Broadcast(responseLatestMsg())
			}
		} else if len(receivedBlocks) == 1 {
			log.Print("We have to query the chain from our peer")
			Broadcast(queryAllMsg())
		} else {
			log.Print("Received blockchain is longer than current blockchain")
			block.ReplaceChain(receivedBlocks)
		}
	} else {
		log.Print("received blockchain is not longer than received blockchain. Do nothing")
	}
}

func BroadcastLatest() {
	Broadcast(responseLatestMsg())
}

func BroadCastTransactionPool() {
	Broadcast(responseTransactionPoolMsg())
}

func ConnectToPeer(newPeer string) {
	u := url.URL{Scheme: "ws", Host: newPeer, Path: "/"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}

	InitConnection(c)
}

func GetSockets() []*websocket.Conn {
	return sockets
}
