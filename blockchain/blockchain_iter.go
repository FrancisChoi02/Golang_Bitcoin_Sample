package blockchain

import (
	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
)

// BlockChainIterator 区块链迭代器
type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

// Iterator 为区块链对象构建对应的迭代器
func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{chain.LastHash, chain.Database}

	return iter
}

// Next 在数据库中找到对应的区块数据，并返回前一个区块反序列化后的数据
func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		zap.L().Error("txn.Get() failed", zap.Error(err))

		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)

		return err
	})
	zap.L().Error("iter.Database.View() failed", zap.Error(err))

	//迭代
	iter.CurrentHash = block.PrevBlockHash

	return block
}
