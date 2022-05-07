package webclient

import (
	"context"
	"ethereum-block-indexer/database"
	"log"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var client *ethclient.Client
var chainID *big.Int
var newestBlockNumber uint64
var quit chan struct{}
var wg sync.WaitGroup
var unstableBlock uint64 = 20

func Init(url string) {
	var err error
	//client, err = ethclient.Dial("wss://data-seed-prebsc-2-s3.binance.org:8545/")
	client, err = ethclient.Dial(url)
	if err != nil {
		log.Fatalf("[webclient] dial error: %s\n", err.Error())
	}
	chainID, err = client.NetworkID(context.Background())
	if err != nil {
		log.Fatalf("[webclient] rpc NetworkID() error: %s\n", err.Error())
	}
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatalf("[webclient] rpc HeaderByNumber() error: %s\n", err.Error())
	}
	newestBlockNumber = header.Number.Uint64()
	log.Printf("[webclient] the newest block number: %d\n", newestBlockNumber)
	quit = make(chan struct{})
	wg.Add(1)
	go subscribeNewHead()
}

func Fini() {
	quit <- struct{}{}
	wg.Wait()
}

// 訂閱當前交易所最新區塊
func subscribeNewHead() {
	headers := make(chan *types.Header)
	sub, err := client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Println(err.Error())
	}
	for {
		select {
		case err := <-sub.Err():
			log.Println(err.Error())
		case header := <-headers:
			atomic.StoreUint64(&newestBlockNumber, header.Number.Uint64())
			log.Printf("[webclient] received new block head %d\n", atomic.LoadUint64(&newestBlockNumber))
		case <-quit:
			wg.Done()
			return
		}
	}
}

// 取得當前交易所最新區塊編號
func GetNewestBlockNnumber() uint64 {
	return atomic.LoadUint64(&newestBlockNumber)
}

// 取得單一 block 的所有資訊
func GetBlockData(number *big.Int) (*database.BlockData, error) {
	log.Printf("[webclient] read block %d from web\n", number)
	var data database.BlockData
	block, err := client.BlockByNumber(context.Background(), number)
	if err != nil {
		log.Printf("[webclient] rpc BlockByNumber() error: %s\n", err.Error())
		return &database.BlockData{Block: &database.EthBlock{BlockNum: number.Uint64()}}, err
	}
	blockNumber := block.NumberU64()
	data.Block = &database.EthBlock{
		BlockNum:   blockNumber,
		BlockHash:  block.Hash().Hex(),
		BlockTime:  block.Time(),
		ParentHash: block.ParentHash().Hex(),
		BlockStable: func() uint8 {
			if block.NumberU64()+unstableBlock <= atomic.LoadUint64(&newestBlockNumber) {
				return 1
			}
			return 0
		}(),
	}
	data.Transactions = make([]*database.BlockTransaction, 0, len(block.Transactions()))
	for _, tx := range block.Transactions() {
		receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
		if err != nil {
			log.Printf("[webclient] rpc TransactionReceipt() error: %s\n", err.Error())
			continue
		}
		data.Logs = make([]*database.TransactionLog, 0, len(receipt.Logs))
		for _, log := range receipt.Logs {
			data.Logs = append(data.Logs, &database.TransactionLog{
				BlockNum: blockNumber,
				TxHash:   tx.Hash().Hex(),
				LogIndex: uint32(log.Index),
				LogData:  log.Data,
			})
		}
		msg, err := tx.AsMessage(types.NewEIP155Signer(chainID), nil)
		if err != nil {
			log.Printf("[webclient] type AsMessage() error: %s\n", err.Error())
			continue
		}
		data.Transactions = append(data.Transactions,
			&database.BlockTransaction{
				BlockNum: blockNumber,
				TxHash:   tx.Hash().Hex(),
				TxFrom:   msg.From().String(),
				TxTo: func() string {
					if msg.To() != nil {
						return msg.To().String()
					}
					return ""
				}(),
				TxNonce: msg.Nonce(),
				TxData:  msg.Data(),
				TxValue: func() uint64 {
					if msg.Value() != nil {
						return msg.Value().Uint64()
					}
					return 0
				}(),
			})
	}
	return &data, nil
}
