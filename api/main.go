package main

import (
	"encoding/json"
	"ethereum-block-api/database"
	"ethereum-block-api/webserver"
	"io"
	"log"
	"os"
	"time"
)

type Config struct {
	DSN  string `json:"dsn"`
	Port string `json:"port"`
}

func main() {
	fout, err := os.Create("ethereum-block-api_" + time.Now().Format("20060102150405") + ".log")
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
	webserver.Init(config.Port)
	quit := make(chan os.Signal, 1)
	<-quit
	webserver.Fini()
	database.Fini()
	return
}
