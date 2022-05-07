package webserver

import (
	"context"
	"encoding/json"
	"ethereum-block-api/database"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var service *http.Server

func Init(port string) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/blocks/", getBlocksByLimit)
	router.GET("/blocks/:id", getBlockByID)
	router.GET("/transaction/:txHash", getTransactionByTXHash)
	service = &http.Server{Addr: port, Handler: router}
	go func() {
		log.Printf("[webserver] start api service\n")
		if err := service.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
}

func Fini() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.Shutdown(ctx); err != nil {
		log.Fatalf("Service api shutdown: %s \n", err)
	}
}

func getBlocksByLimit(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	log.Printf("[webserver] request BlocksByLimi(%d)", limit)
	// [TODO] 應增加最大數量限制
	result := database.QueryBlocksByLimit(limit)
	c.JSON(200, result)
	data, err := json.Marshal(result)
	if err == nil {
		log.Printf("[webserver] response BlocksByLimit(%d):\n%s\n", limit, string(data))
	}
}

func getBlockByID(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	log.Printf("[webserver] request BlockByID(%d)", id)
	// [TODO] 可增加 id 範圍檢查
	result := database.QueryBlockByID(id)
	c.JSON(200, result)
	data, err := json.Marshal(result)
	if err == nil {
		log.Printf("[webserver] response BlockByID(%d):\n%s\n", id, string(data))
	}
}

func getTransactionByTXHash(c *gin.Context) {
	txHash := c.Param("txHash")
	log.Printf("[webserver] request TransactionByTXHash(%s)", txHash)
	// [TODO] 可增加 txHash 基本格式檢查
	result := database.QueryTransactionByTXHash(txHash)
	c.JSON(200, result)
	data, err := json.Marshal(result)
	if err == nil {
		log.Printf("[webserver] response TransactionByTXHash(%s):\n%s\n", txHash, string(data))
	}
}
