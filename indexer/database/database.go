package database

import (
	"log"
	"math"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type EthBlock struct {
	BlockNum    uint64
	BlockHash   string
	BlockTime   uint64
	ParentHash  string
	BlockStable uint8
}

type BlockTransaction struct {
	BlockNum uint64
	TxHash   string
	TxFrom   string
	TxTo     string
	TxNonce  uint64
	TxData   []byte
	TxValue  uint64
}

type TransactionLog struct {
	BlockNum uint64
	TxHash   string
	LogIndex uint32
	LogData  []byte
}

type BlockData struct {
	Block        *EthBlock
	Transactions []*BlockTransaction
	Logs         []*TransactionLog
}

var db *gorm.DB

func Init(dsn string) {
	var err error
	//dsn := "blockchain:blockchain@tcp(127.0.0.1:3306)/eth?charset=utf8mb4&parseTime=True&loc=Local"
	if db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{CreateBatchSize: 1000, NamingStrategy: schema.NamingStrategy{SingularTable: true}}); err != nil {
		log.Fatalf("[database] open db error: %s\n", err.Error())
	}
	db = db.Session(&gorm.Session{CreateBatchSize: 1000})
	db.Where("block_stable = ?", 0).Delete(&EthBlock{})
}

func Fini() {
}

func QueryMaxBlockNumber() uint64 {
	number := uint64(0)
	row := db.Table("eth_block").Select("IFNULL(MAX(block_num),0)").Row()
	err := row.Scan(&number)
	if err != nil {
		log.Printf("[database] queryMaxBlockNumber DB error: %s\n", err.Error())
	}
	return number
}

func QueryMinUnstableBlockNumber() uint64 {
	number := uint64(math.MaxUint64)
	row := db.Table("eth_block").Where("block_stable = ?", 0).Select("IFNULL(MIN(block_num),18446744073709551615)").Row()
	err := row.Scan(&number)
	if err != nil {
		log.Printf("[database] queryMinUnstableBlockNumber DB error: %s\n", err.Error())
	}
	return number
}

func BulkInsertBlockData(data []*BlockData) {
	blocks := make([]*EthBlock, 0, len(data))
	transactions := make([]*BlockTransaction, 0, len(data))
	logs := make([]*TransactionLog, 0, len(data))
	for _, block := range data {
		log.Printf("[database] write block %d to DB\n", block.Block.BlockNum)
		blocks = append(blocks, block.Block)
		transactions = append(transactions, block.Transactions...)
		logs = append(logs, block.Logs...)
		block = nil
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(blocks, len(blocks)).Error; err != nil {
		log.Printf("[database] bulkInsertBlockData to eth_block error: %s\n", err.Error())
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(transactions, len(transactions)).Error; err != nil {
		log.Printf("[database] bulkInsertBlockData to block_transaction error: %s\n", err.Error())
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(logs, len(logs)).Error; err != nil {
		log.Printf("[database] bulkInsertBlockData to transaction_log error: %s\n", err.Error())
	}
}

func UpsertBlockData(data *BlockData) {
	log.Printf("[database] fix stable block %d to DB\n", data.Block.BlockNum)
	db.Where("block_num = ?", data.Block.BlockNum).Delete(&EthBlock{})
	db.Create(data.Block)
	db.Create(data.Transactions)
	db.Create(data.Logs)
}
