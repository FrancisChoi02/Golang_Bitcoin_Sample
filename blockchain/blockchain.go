package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	dbPath      = "./tmp/blocks_%s"
	genesisData = "First Transaction from Genesis"
)

// 区块链对象
type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

// DBexists 查看数据库是否存在
func DBexists(path string) bool {
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}

	return true
}

// retry 数据库启动辅助函数
func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err)
	}
	retryOpts := originalOpts
	retryOpts.Truncate = true
	db, err := badger.Open(retryOpts)
	return db, err
}

// openDB 打开数据库
func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(dir, opts); err == nil {
				log.Println("database unlocked, value log truncated")
				return db, nil
			}
			log.Println("could not unlock database:", err)
		}
		return nil, err
	} else {
		return db, nil
	}
}

// ContinueBlockChain
func ContinueBlockChain(nodeId string) *BlockChain {
	//查看当前节点对应的数据库是否存在
	path := fmt.Sprintf(dbPath, nodeId)
	if DBexists(path) == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	//数据库启动
	opts := badger.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path

	db, err := openDB(path, opts)
	zap.L().Error("openDB() failed", zap.Error(err))

	//获取最新区块的哈希值
	var lastHash []byte
	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		zap.L().Error("txn.Get() failed", zap.Error(err))

		lastHash, err = item.Value()
		return err
	})
	zap.L().Error("db.Update() failed", zap.Error(err))

	//构建并返回区块链对象
	chain := BlockChain{lastHash, db}
	return &chain
}

// MineBlock 构造候选区块
func (chain *BlockChain) MineBlock(transactions []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int

	for _, tx := range transactions {
		if chain.VerifyTransaction(tx) != true {
			log.Panic("Invalid Transaction")
		}
	}

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		zap.L().Error("txn.Get() failed", zap.Error(err))
		lastHash, err = item.Value()

		item, err = txn.Get(lastHash)
		zap.L().Error("txn.Get() failed", zap.Error(err))
		lastBlockData, _ := item.Value()

		lastBlock := Deserialize(lastBlockData)

		lastHeight = lastBlock.Height

		return err
	})
	zap.L().Error("chain.Database.View() failed", zap.Error(err))

	newBlock := CreateBlock(transactions, lastHash, lastHeight+1)

	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		zap.L().Error("txn.Set() failed", zap.Error(err))

		err = txn.Set([]byte("lh"), newBlock.Hash)

		chain.LastHash = newBlock.Hash

		return err
	})
	zap.L().Error("chain.Database.Update() failed", zap.Error(err))

	return newBlock
}

// InitBlockChain 创建区块链对象
func InitBlockChain(address, nodeId string) *BlockChain {
	//查看当前节点对应的数据库是否存在
	path := fmt.Sprintf(dbPath, nodeId)
	if DBexists(path) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path

	//数据库启动
	db, err := openDB(path, opts)
	zap.L().Error("openDB() failed", zap.Error(err))

	//构建并记录区块链对象的相关
	var lastHash []byte
	err = db.Update(func(txn *badger.Txn) error {

		// 构建Coinbase交易和创世区块
		cbtx := CoinbaseTx(address, genesisData)
		genesis := GenesisBlock(cbtx)
		fmt.Println("Genesis created")
		err = txn.Set(genesis.Hash, genesis.Serialize()) //将创世区块的信息记录到数据库中

		zap.L().Error("txn.Get() failed", zap.Error(err))
		err = txn.Set([]byte("lh"), genesis.Hash)

		lastHash = genesis.Hash

		return err

	})
	zap.L().Error("db.Update() failed", zap.Error(err))

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

// FindTransaction 根据Id查询交易对象
func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
	// 区块迭代器初始化
	iter := bc.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		// 如果到创世区块都还没找到交易，则退出循环、报错
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction does not exist")
}

// SignTransaction 签署交易
func (bc *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		// 寻找交易是否存在，并获 取交易对象
		prevTX, err := bc.FindTransaction(in.ID)
		zap.L().Error("bc.FindTransaction failed()", zap.Error(err))

		// 将交易ID与交易对象的索引关系通过map保存
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

// VerifyTransaction 验证交易合法性
func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	if tx.IsCoinbaseTx() {
		return true
	}
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		zap.L().Error("bc.FindTransaction() failed", zap.Error(err))

		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}

// FindUTXO 查找UTXO
func (chain *BlockChain) FindUTXO() map[string]TxOutputs {
	UTXO := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)

	//初始化区块链迭代器
	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

			//查找有哪些输出结构的交易，没有被输入结构引用
		Outputs:
			for outIdx, out := range tx.Outputs {
				// 查找本交易是否在输入结构索引中
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}

				//如果输出结构中的交易没有被使用过，则添加到UTXO集合中
				//UTXO也是交易ID 和 未使用Output的关系映射
				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			// 当交易类型并非币基交易时，标记输入结构中使用过的UTXO
			if tx.IsCoinbaseTx() == false {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					// 标记使用过的交易对应的输出索引
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
				}
			}
		}

		//遍历到创世区块后，跳出循环
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
	return UTXO
}

// GetBestHeight 获取区块当前最高的区块高度
func (chain *BlockChain) GetBestHeight() int {
	var lastBlock Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		zap.L().Error("txn.Get() failed", zap.Error(err))
		lastHash, _ := item.Value()

		item, err = txn.Get(lastHash)
		zap.L().Error("txn.Get() failed", zap.Error(err))
		lastBlockData, _ := item.Value()

		lastBlock = *Deserialize(lastBlockData)

		return nil
	})
	zap.L().Error("chain.Database.View() failed", zap.Error(err))

	return lastBlock.Height
}

// AddBlock 向本地区块链中添加区块
func (chain *BlockChain) AddBlock(block *Block) {
	err := chain.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err == nil {
			return nil
		}

		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		zap.L().Error("txn.Set() failed", zap.Error(err))

		item, err := txn.Get([]byte("lh"))
		zap.L().Error("txn.Get() failed", zap.Error(err))
		lastHash, _ := item.Value()

		item, err = txn.Get(lastHash)
		zap.L().Error("txn.Get() failed", zap.Error(err))
		lastBlockData, _ := item.Value()

		lastBlock := Deserialize(lastBlockData)

		if block.Height > lastBlock.Height {
			err = txn.Set([]byte("lh"), block.Hash)
			zap.L().Error("txn.Set() failed", zap.Error(err))
			chain.LastHash = block.Hash
		}

		return nil
	})
	zap.L().Error("chain.Database.Update() failed", zap.Error(err))
}

// GetBlock 从数据库中获取指定哈希值的区块
func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("Block is not found")
		} else {
			blockData, _ := item.Value()

			block = *Deserialize(blockData)
		}
		return nil
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

// GetBlockHashes 获取当前区块链中所有区块的哈希值列表
func (chain *BlockChain) GetBlockHashes() [][]byte {
	var blocks [][]byte

	iter := chain.Iterator()

	for {
		block := iter.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return blocks
}
