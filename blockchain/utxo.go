package blockchain

import (
	"bytes"
	"encoding/hex"
	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
	"log"
)

// 定义utxo键值对前缀
var (
	utxoPrefix   = []byte("utxo-")
	prefixLength = len(utxoPrefix)
)

type UTXOSet struct {
	Blockchain *BlockChain
}

// FindSpendableOutputs 查找可以使用的UTXO
func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.Database

	err := db.View(func(txn *badger.Txn) error {
		// 初始化数据库迭代器
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历整个UTXO集合，将与指定公钥哈希相匹配的未花费输出，并返回使用到的未花费输出切片
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key()
			v, err := item.Value()
			zap.L().Error("item.Value()", zap.Error(err))

			//除去键值对前缀，获取交易ID
			k = bytes.TrimPrefix(k, utxoPrefix)
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)

			for outIdx, out := range outs.Outputs {
				if out.PubKeyHashEquals(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
			}
		}
		return nil
	})
	zap.L().Error("db.View()", zap.Error(err))

	return accumulated, unspentOuts
}

// FindAddressBalance 通过地址的UTXO计算余额
func (u UTXOSet) FindAddressBalance(pubKeyHash []byte) []TxOutput {
	var UTXOs []TxOutput

	db := u.Blockchain.Database

	err := db.View(func(txn *badger.Txn) error {
		// 初始化数据库迭代器
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			v, err := item.Value()
			zap.L().Error("item.Value()", zap.Error(err))

			outs := DeserializeOutputs(v)
			for _, out := range outs.Outputs {
				// 获取属于该地址的UTXO集合
				if out.PubKeyHashEquals(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}

		}
		return nil
	})
	zap.L().Error("db.View()", zap.Error(err))

	return UTXOs
}

// CountTransactions 统计UTXO的数量
func (u UTXOSet) CountTransactions() int {
	db := u.Blockchain.Database
	counter := 0

	err := db.View(func(txn *badger.Txn) error {
		// 初始化数据库迭代器
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历统计所有合法的UTXO
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}

		return nil
	})

	zap.L().Error("db.View()", zap.Error(err))

	return counter
}

// Reindex 更新数据库中的UTXO集合
func (u UTXOSet) Reindex() {
	// 删除数据库中的UTXO键值对
	db := u.Blockchain.Database
	u.DeleteByPrefix(utxoPrefix)

	// 从区块链中获取新的UTXO集合
	UTXO := u.Blockchain.FindUTXO()

	// 将UTXO集合持久化到数据库中
	err := db.Update(func(txn *badger.Txn) error {
		for txId, outs := range UTXO {
			key, err := hex.DecodeString(txId)
			zap.L().Error("hex.DecodeString()", zap.Error(err))

			key = append(utxoPrefix, key...)

			err = txn.Set(key, outs.SerializeOutputs())
			zap.L().Error("txn.Set()", zap.Error(err))
		}

		return nil
	})
	zap.L().Error("db.Update()", zap.Error(err))
}

// Update UTXO集合更新
func (u *UTXOSet) Update(block *Block) {
	db := u.Blockchain.Database

	err := db.Update(func(txn *badger.Txn) error {

		// 遍历区块中的所有交易
		for _, tx := range block.Transactions {

			// 若当前交易非币基交易，遍历所有Input，删除使用过的UTXO
			if tx.IsCoinbaseTx() == false {
				for _, in := range tx.Inputs {
					// 获取输入结构中记录的交易Id，获取本交易对应的UTXO集合
					inID := append(utxoPrefix, in.ID...)
					item, err := txn.Get(inID)
					zap.L().Error("txn.Get()", zap.Error(err))

					tmpUTXO, err := item.Value()
					zap.L().Error("item.Value()", zap.Error(err))

					updatedOuts := TxOutputs{}
					outs := DeserializeOutputs(tmpUTXO)
					for outIdx, out := range outs.Outputs {
						//删除与当前输入相关的未花费交易输出(UTXO)，过滤掉被使用过的UTXO
						if outIdx != in.Out {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}

					// 如果所有UTXO都被使用过，则将该交易对应的UTXO集合删除
					if len(updatedOuts.Outputs) == 0 {
						if err := txn.Delete(inID); err != nil {
							zap.L().Error("txn.Delete()", zap.Error(err))
							log.Panic(err)
						}
					} else {
						// 更新交易ID对应的UTXO键值对
						if err := txn.Set(inID, updatedOuts.SerializeOutputs()); err != nil {
							zap.L().Error("txn.Set()", zap.Error(err))
							log.Panic(err)
						}
					}
				}
			}

			// 添加新的UTXO
			newOutputs := TxOutputs{}
			for _, out := range tx.Outputs {
				newOutputs.Outputs = append(newOutputs.Outputs, out)
			}

			txID := append(utxoPrefix, tx.ID...)
			if err := txn.Set(txID, newOutputs.SerializeOutputs()); err != nil {
				log.Panic(err)
			}
		}

		return nil
	})
	zap.L().Error("db.Update()", zap.Error(err))
}

// DeleteByPrefix 将响应键值对前缀的数据删除
func (u *UTXOSet) DeleteByPrefix(prefix []byte) {
	// 初始化用于删除键的匿名函数
	deleteKeys := func(keysForDelete [][]byte) error {
		if err := u.Blockchain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					zap.L().Error("txn.Delete() failed", zap.Error(err))
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}

	collectSize := 100000
	u.Blockchain.Database.View(func(txn *badger.Txn) error {
		// 数据库迭代器初始化
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			// 遍历并获取需要删除的键值对
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++

			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					zap.L().Error(" deleteKeys()", zap.Error(err))
				}
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}

		// 如果删除集合不为空
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				zap.L().Error(" deleteKeys()", zap.Error(err))
			}
		}
		return nil
	})
}
