package blockchain

import (
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
