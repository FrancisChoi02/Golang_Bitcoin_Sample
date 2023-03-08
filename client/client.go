package client

import (
	"Golang_Bitcoin_Sample/wallet"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
)

type CommandLine struct{}

// printUsage 打印所有功能
func (client *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" send -from 转账地址 -to 接收地址 -amount 转账数目 -mine 是否挖矿- 进行代币转账。如果设置了-mine参数，则打包该区块进行挖矿")
}

// validateArgs() 检测输入的参数个数
func (client *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		client.printUsage()
		runtime.Goexit()
	}
}

// createWallet 生成钱包
func (client *CommandLine) createWallet(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeID)

	fmt.Printf("New address is: %s\n", address)
}

// Run 客户端运行客户端
func (client *CommandLine) Run() {
	client.validateArgs()

	//从环境变量中获取节点的编号
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		fmt.Printf("NODE_ID env is not set!")
		runtime.Goexit()
	}

	//获取调用的具体方法
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)

	// 判断调用的方法类型
	switch os.Args[1] {
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}

	default:
		fmt.Println("方法调用错误")
		runtime.Goexit()
	}

	if createWalletCmd.Parsed() {
		client.createWallet(nodeID)
	}

}
