package client

import (
	"Golang_Bitcoin_Sample/blockchain"
	"Golang_Bitcoin_Sample/wallet"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
)

type CommandLine struct{}

// printUsage 打印所有功能
func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" getbalance -address 钱包地址 - 获取地址的余额")
	fmt.Println(" createblockchain -address 钱包地址 -创建一条区块链并发放一笔创世区块奖励至地址中")
	fmt.Println(" printchain - 遍历区块链")
	fmt.Println(" send -from 转账地址 -to 接收地址 -amount 转账数目 -mine 挖矿- 发送一定数量的代币，如果设置了-mine标志，则从该节点挖掘")
	fmt.Println(" createwallet - 创建钱包地址")
	fmt.Println(" listaddresses - 展示钱包文件中的所有钱包地址")
	fmt.Println(" reindexutxo - 更新UTXO集合")
	fmt.Println(" startnode -miner ADDRESS - 使用 NODE_ID 环境变量指定的 ID 启动节点。-miner 选项启用挖矿。")
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

// listAddresses 列出对应节点下保存的所有钱包文件
func (cli *CommandLine) listAddresses(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	addresses := wallets.GetAllAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}

}

// createBlockChain 在当前的节点下创建区块链对象，并获得创世区块奖励
func (cli *CommandLine) createBlockChain(address, nodeID string) {
	//验证钱包有效性
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not Valid")
	}
	chain := blockchain.InitBlockChain(address, nodeID)
	defer chain.Database.Close()

	//UTXO集合操作

	fmt.Println("Finished!")
}

// printChain 遍历区块链
func (cli *CommandLine) printChain(nodeID string) {
	//获取对应节点下的区块链对象，并初始化迭代器
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	iter := chain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("Hash: %x\n", block.Hash)
		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)

		//验证区块的合法性
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		fmt.Println()

		//遍历到创世区块后停止
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
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
	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)

	// 判断调用的方法类型
	switch os.Args[1] {
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "listaddresses":
		err := listAddressesCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createblockchain":
		err := createBlockchainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
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
