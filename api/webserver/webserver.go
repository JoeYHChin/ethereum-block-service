package webserver

import (
	"context"
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
	// [TODO] 應增加最大數量限制
	c.JSON(200, database.QueryBlocksByLimit(limit))
}

func getBlockByID(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// [TODO] 可增加 id 範圍檢查
	c.JSON(200, database.QueryBlockByID(id))
}

func getTransactionByTXHash(c *gin.Context) {
	txHash := c.Param("txHash")
	// [TODO] 可增加 txHash 基本格式檢查
	c.JSON(200, database.QueryTransactionByTXHash(txHash))
}
