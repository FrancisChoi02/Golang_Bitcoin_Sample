package client

import (
	"Golang_Bitcoin_Sample/blockchain"
	"Golang_Bitcoin_Sample/network"
	"Golang_Bitcoin_Sample/wallet"
	"flag"
	"fmt"
	"go.uber.org/zap"
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
	// 验证钱包有效性
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not Valid")
	}
	// 初始化区块链对象，并获得创世区块的区块收益
	chain := blockchain.InitBlockChain(address, nodeID)
	defer chain.Database.Close()

	// 更新数据库中的UTXO集合
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

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

// send 转账交易
func (cli *CommandLine) send(from, to string, amount int, nodeID string, mineNow bool) {
	//判断参与转账的地址的有效性
	if !wallet.ValidateAddress(to) {
		zap.L().Error("To-Address is not Valid")
		return
	}
	if !wallet.ValidateAddress(from) {
		zap.L().Error("From-Address is not Valid")
		return
	}

	// 获取区块链对象、UTXO集对象
	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{chain}
	defer chain.Database.Close()

	// 从钱包文件中获取钱包集合，并通过地址获取具体钱包对象
	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		zap.L().Error("wallet.CreateWallets()", zap.Error(err))
		return
	}
	wallet := wallets.GetWallet(from)

	// 创建交易对象
	tx := blockchain.NewTransaction(&wallet, to, amount, &UTXOSet)

	// 根据mineNow标记判断交易的处理方法
	if mineNow {
		// 将所有交易打包到候选区块中，开始挖矿
		cbTx := blockchain.CoinbaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)

		//更新UTXO集合
		UTXOSet.Update(block)
	} else {
		//广播交易
		fmt.Println("send tx")
	}

	fmt.Println("Success!")
}

// getBalance 获取当前地址还有多少UTXO
func (cli *CommandLine) getBalance(address, nodeID string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not Valid")
	}
	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{chain}
	defer chain.Database.Close()

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	UTXOs := UTXOSet.FindAddressBalance(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of %s: %d\n", address, balance)
}

// reindexUTXO 更新本地的UTXO集合
func (cli *CommandLine) reindexUTXO(nodeID string) {
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("UTXO 集合中有 %d 笔交易.\n", count)
}

// StartNode 开启节点通信功能
func (cli *CommandLine) StartNode(nodeID, minerAddress string) {
	fmt.Printf("Starting Node %s\n", nodeID)

	//判断钱包是否合法
	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("接收出块奖励的地址为: ", minerAddress)
		} else {
			log.Panic("地址格式不合法")
		}
	}
	network.StartServer(nodeID, minerAddress)
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

	// 获取调用的具体方法
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)

	// 命令行参数解析与获取
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	createBlockchainAddress := createBlockchainCmd.String("address", "", "The address to send genesis block reward to")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	sendMine := sendCmd.Bool("mine", false, "Mine immediately on the same node")
	startNodeMiner := startNodeCmd.String("miner", "", "Enable mining mode and send reward to ADDRESS")

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
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "reindexutxo":
		err := reindexUTXOCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		fmt.Println("方法调用错误")
		runtime.Goexit()
	}

	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			runtime.Goexit()
		}
		client.getBalance(*getBalanceAddress, nodeID)
	}

	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			runtime.Goexit()
		}
		client.createBlockChain(*createBlockchainAddress, nodeID)
	}

	if printChainCmd.Parsed() {
		client.printChain(nodeID)
	}

	if createWalletCmd.Parsed() {
		client.createWallet(nodeID)
	}
	if listAddressesCmd.Parsed() {
		client.listAddresses(nodeID)
	}
	if reindexUTXOCmd.Parsed() {
		client.reindexUTXO(nodeID)
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}

		client.send(*sendFrom, *sendTo, *sendAmount, nodeID, *sendMine)
	}

	if startNodeCmd.Parsed() {
		nodeID := os.Getenv("NODE_ID")
		if nodeID == "" {
			startNodeCmd.Usage()
			runtime.Goexit()
		}
		client.StartNode(nodeID, *startNodeMiner)
	}

}
