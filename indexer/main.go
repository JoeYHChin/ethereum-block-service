package main

import (
	"encoding/json"
	"ethereum-block-indexer/database"
	"ethereum-block-indexer/webclient"
	"io"
	"log"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	DSN        string `json:"dsn"`
	Endpoint   string `json:"endpoint"`
	StartBlock uint64 `json:"startblock"`
	Parallel   int    `json:"parallel"`
}

var newestBlockNnumber uint64
var latestBlockNumber uint64
var latestUnstableBlockNumber uint64
var buffer []*database.BlockData
var mux sync.RWMutex
var wg sync.WaitGroup
var quit uint32
var unstableBlock uint64 = 20

func main() {
	fout, err := os.Create("ethereum-block-indexer_" + time.Now().Format("20060102150405") + ".log")
	if err != nil {
		panic(err)
	}
	defer fout.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, fout))
	configuration, err := os.Open("config.json")
	if err != nil {
		log.Fatalf("[main] open config error: %s\n", err.Error())
	}
	defer configuration.Close()
	var config Config
	if err = json.NewDecoder(configuration).Decode(&config); err != nil {
		log.Fatalf("[main] load config error: %s\n", err.Error())
	}
	database.Init(config.DSN)
	webclient.Init(config.Endpoint)
	latestBlockNumber = database.QueryMaxBlockNumber()
	if latestBlockNumber < config.StartBlock {
		latestBlockNumber = config.StartBlock
	}
	log.Printf("[main] the latest block number: %d\n", latestBlockNumber)
	wg.Add(1)
	go writer()
	for i := 1; i <= config.Parallel; i++ {
		wg.Add(1)
		go reader(uint8(i))
	}
	wg.Add(1)
	go fixer()
	exit := make(chan os.Signal, 1)
	<-exit
	atomic.StoreUint32(&quit, 1)
	wg.Wait()
	webclient.Fini()
	database.Fini()
	return
}

// 從 web3 API 讀取區塊資料至 buffer
func reader(id uint8) {
	number := uint64(0)
	for atomic.LoadUint32(&quit) == 0 {
		if number == 0 {
			number = atomic.AddUint64(&latestBlockNumber, 1)
		}
		if number <= webclient.GetNewestBlockNnumber() {
			data, _ := webclient.GetBlockData(new(big.Int).SetUint64(number))
			mux.Lock()
			buffer = append(buffer, data)
			mux.Unlock()
			number = 0
		} else {
			time.Sleep(10 * time.Millisecond)
		}
		// [TODO] 可透過 id 在追上進度後，將 go routine 減少至 4 以下
	}
	wg.Done()
}

// 將資料從 buffer 寫至 DB
func writer() {
	buffer = make([]*database.BlockData, 0, 1000)
	backupBuffer := make([]*database.BlockData, 0, 1000)
	for atomic.LoadUint32(&quit) == 0 {
		mux.Lock()
		if len(buffer) > 0 {
			data := buffer
			buffer = backupBuffer
			mux.Unlock()
			database.BulkInsertBlockData(data)
			backupBuffer = make([]*database.BlockData, 0, 1000)
		} else {
			mux.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
	}
	wg.Done()
}

// 透過 web3 API 讀取不穩定區塊的資料，並更新至 DB
func fixer() {
	for atomic.LoadUint32(&quit) == 0 {
		number := database.QueryMinUnstableBlockNumber()
		if number <= webclient.GetNewestBlockNnumber()-unstableBlock {
			var w sync.WaitGroup
			for i := number; i <= webclient.GetNewestBlockNnumber()-unstableBlock; i++ {
				w.Add(1)
				go func(number *big.Int) {
					data, err := webclient.GetBlockData(number)
					if err == nil {
						database.UpsertBlockData(data)
					}
					w.Done()
				}(new(big.Int).SetUint64(i))
			}
			w.Wait()
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
	wg.Done()
}
