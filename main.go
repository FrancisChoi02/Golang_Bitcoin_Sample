package main

import (
	"Golang_Bitcoin_Sample/client"
	"Golang_Bitcoin_Sample/logger"
	"fmt"
	"os"
)

func main() {
	defer os.Exit(0)

	//加载日志器
	cfg := logger.LogConfig{
		Level:      "info",
		Filename:   "bitcoin.log",
		MaxSize:    200,
		MaxAge:     30,
		MaxBackups: 7,
	}
	if err := logger.Init(cfg, "dev"); err != nil {
		fmt.Printf("init logger failed, err:%v\n", err)
		return
	}

	//运行客户端
	cmd := client.CommandLine{}
	cmd.Run()
}
