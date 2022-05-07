package database

import (
	"encoding/hex"
	"log"
	"math"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type EthBlock struct {
	BlockNum    uint64 `json:"block_num"`
	BlockHash   string `json:"block_hash"`
	BlockTime   uint64 `json:"block_time"`
	ParentHash  string `json:"parent_hash"`
	BlockStable uint8  `json:"stable"`
}

type BlockTransaction struct {
	TxHash   string
	BlockNum uint64
	TxFrom   string
	TxTo     string
	TxNonce  uint64
	TxData   []byte
	TxValue  uint64
}

type TransactionLog struct {
	TxHash   string
	LogIndex uint32
	LogData  []byte
}

type BlockWithTransactions struct {
	BlockNum   uint64    `json:"block_num"`
	BlockHash  string    `json:"block_hash"`
	BlockTime  uint64    `json:"block_time"`
	ParentHash string    `json:"parent_hash"`
	TxHashs    []*string `json:"transactions"`
}

type TransactionWithLogs struct {
	TxHash  string `json:"tx_hash"`
	TxFrom  string `json:"from"`
	TxTo    string `json:"to"`
	TxNonce uint64 `json:"nonce"`
	TxData  string `json:"data"`
	TxValue uint64 `json:"value"`
	Logs    []*struct {
		LogIndex uint32 `json:"index"`
		LogData  string `json:"data"`
	} `json:"logs"`
}

type cache struct {
	expiratoin int64
	content    interface{}
}

var db *gorm.DB
var cacheBlocksByLimit *lru.Cache
var cacheBlockByID *lru.Cache
var cacheTransactionByTXHash *lru.Cache

func Init(dsn string) {
	var err error
	//dsn := "blockchain:blockchain@tcp(127.0.0.1:3306)/eth?charset=utf8mb4&parseTime=True&loc=Local"
	if db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{CreateBatchSize: 1000, NamingStrategy: schema.NamingStrategy{SingularTable: true}}); err != nil {
		log.Fatalf("[database] open db error: %s", err.Error())
	}
	db = db.Session(&gorm.Session{CreateBatchSize: 1000})
	cacheBlocksByLimit, err = lru.New(1024)
	if err != nil {
		log.Fatalf("[database] create cacheBlocksByLimit error: %s", err.Error())
	}
	cacheBlockByID, err = lru.New(1024)
	if err != nil {
		log.Fatalf("[database] create cacheBlockByID error: %s", err.Error())
	}
	cacheTransactionByTXHash, err = lru.New(1024)
	if err != nil {
		log.Fatalf("[database] cacheTransactionByTXHash cache error: %s", err.Error())
	}
}

func Fini() {
}

func QueryBlocksByLimit(n int) []EthBlock {
	// 讀取 LRU 快取，檢查是否失效
	if value, ok := cacheBlocksByLimit.Get(n); ok {
		if time.Now().Unix() <= value.(cache).expiratoin {
			return value.(cache).content.([]EthBlock)
		}
	}
	blocks := make([]EthBlock, 0, n)
	db.Order("block_num desc").Limit(n).Find(&blocks)
	// 儲存 LRU 快取，因為前 n 塊必含有不穩定區塊，因此只保留一秒
	cacheBlocksByLimit.Add(n, cache{time.Now().Unix() + 1, blocks})
	return blocks
}

func QueryBlockByID(id uint64) *BlockWithTransactions {
	// 讀取 LRU 快取，因為有不穩定區塊的存在，所以要檢查是否失效
	if value, ok := cacheBlockByID.Get(id); ok {
		if time.Now().Unix() <= value.(cache).expiratoin {
			return value.(cache).content.(*BlockWithTransactions)
		}
	}
	block := EthBlock{}
	db.Where("block_num = ?", id).Find(&block)
	result := BlockWithTransactions{
		BlockNum:   block.BlockNum,
		BlockHash:  block.BlockHash,
		BlockTime:  block.BlockTime,
		ParentHash: block.ParentHash,
		TxHashs:    make([]*string, 0, 100)}
	rows, err := db.Debug().Table("block_transaction").Where("block_num = ?", id).Select("tx_hash").Rows()
	if err != nil {
		log.Printf("[database] queryBlockByID DB error: %s", err.Error())
		return &result
	}
	defer rows.Close()
	for rows.Next() {
		txHash := ""
		err = rows.Scan(&txHash)
		if err != nil {
			log.Printf("[database] queryBlockByID DB error: %s", err.Error())
			return &result
		}
		result.TxHashs = append(result.TxHashs, &txHash)
	}
	// 儲存 LRU 快取，不穩定區塊保留一秒，穩定區塊則等容量滿了自動清除
	if block.BlockStable > 0 {
		cacheBlockByID.Add(id, cache{math.MaxInt64, &result})
	} else {
		cacheBlockByID.Add(id, cache{time.Now().Unix() + 1, &result})
	}
	return &result
}

func QueryTransactionByTXHash(txHash string) *TransactionWithLogs {
	if value, ok := cacheTransactionByTXHash.Get(txHash); ok {
		if time.Now().Unix() <= value.(cache).expiratoin {
			return value.(cache).content.(*TransactionWithLogs)
		}
	}
	transaction := BlockTransaction{}
	db.Where("tx_hash = ?", txHash).Find(&transaction)
	result := TransactionWithLogs{
		TxHash:  transaction.TxHash,
		TxFrom:  transaction.TxFrom,
		TxTo:    transaction.TxTo,
		TxNonce: transaction.TxNonce,
		TxData:  "0x" + hex.EncodeToString(transaction.TxData),
		TxValue: transaction.TxValue,
		Logs: make([]*struct {
			LogIndex uint32 `json:"index"`
			LogData  string `json:"data"`
		}, 0, 100),
	}
	rows, err := db.Debug().Table("transaction_log").Where("tx_hash = ?", txHash).Select("transaction_log.log_index,transaction_log.log_data,eth_block.block_stable").Joins("JOIN eth_block on transaction_log.block_num = eth_block.block_num").Rows()
	if err != nil {
		log.Printf("[database] queryTransactionByTXHash DB error: %s", err.Error())
		return &result
	}
	defer rows.Close()
	blockStable := uint8(0)
	for rows.Next() {
		logIndex := uint32(0)
		logData := make([]byte, 0, 1000)
		err = rows.Scan(&logIndex, &logData, &blockStable)
		if err != nil {
			log.Printf("[database] queryTransactionByTXHash DB error: %s", err.Error())
			return &result
		}
		result.Logs = append(result.Logs,
			&struct {
				LogIndex uint32 `json:"index"`
				LogData  string `json:"data"`
			}{
				LogIndex: logIndex,
				LogData:  "0x" + hex.EncodeToString(logData),
			})
	}
	// 儲存 LRU 快取，不穩定區塊保留一秒，穩定區塊則等容量滿了自動清除
	if blockStable > 0 {
		cacheTransactionByTXHash.Add(txHash, cache{math.MaxInt64, &result})
	} else {
		cacheTransactionByTXHash.Add(txHash, cache{time.Now().Unix() + 1, &result})
	}
	return &result
}
