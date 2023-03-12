package blockchain

import (
	"bytes"
	"encoding/gob"
	"go.uber.org/zap"
	"time"
)

//区块结构
//Transactions、MerkleRoot、PrevBlockHash、Nonce、Height需要进行哈希计算
type Block struct {
	Timestamp     int64  //时间戳
	Hash          []byte //当前区块哈希值
	MerkleRoot    []byte //默克尔树根
	PrevBlockHash []byte //前块哈希值
	Nonce         int    //随机值

	Transactions []*Transaction
	Height       int //区块高度
}

func (b *Block) GetMerkleRoot() []byte {
	var txHashes [][]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Serialize())
	}
	tree := NewMerkleTree(txHashes)

	return tree.MerkleRoot.Data
}

// CreateBlock 创建区块
func CreateBlock(txs []*Transaction, prevHash []byte, height int) *Block {
	block := &Block{time.Now().Unix(), []byte{}, []byte{}, prevHash, 0, txs, height}

	//挖矿成功后赋值最终的nonce和区块哈希值
	pow := NewProof(block)
	nonce, hash := pow.Run()

	block.Hash = hash
	block.Nonce = nonce

	return block
}

// GenesisBlock 构建创世区块
func GenesisBlock(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{}, 0)
}

// Serialize 区块序列化
func (b *Block) Serialize() []byte {
	var res bytes.Buffer

	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)

	zap.L().Error("encoder.Encode() failed", zap.Error(err))

	return res.Bytes()
}

// Deserialize 区块数据反序列化
func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)

	zap.L().Error("decoder.Decode() failed", zap.Error(err))

	return &block
}
