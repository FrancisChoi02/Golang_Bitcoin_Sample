package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/big"
)

const Difficulty = 12

// 挖矿结构体
type ProofOfWork struct {
	Block  *Block   //候选区块
	Target *big.Int //目标阈值
}

// NewProof 获得带当前目标阈值的挖矿对象
func NewProof(b *Block) *ProofOfWork {
	//获取挖矿难度的目标域值
	target := big.NewInt(1)
	target.Lsh(target, uint(256-Difficulty))

	//计算merkelRoot
	b.MerkleRoot = b.GetMerkleRoot()

	//返回一个挖矿对象
	pow := &ProofOfWork{b, target}
	return pow
}

// InitData 数据准备
func (pow *ProofOfWork) InitData(nonce int) []byte {
	// 将区块头中的数据连接在一起
	data := bytes.Join(
		[][]byte{
			pow.Block.PrevBlockHash, //上一个区块的哈希值
			pow.Block.MerkleRoot,    //默克尔树根
			ToHex(int64(nonce)),
			ToHex(int64(Difficulty)),
		},
		[]byte{},
	)

	return data
}

// Run 随机数计算过程
func (pow *ProofOfWork) Run() (int, []byte) {
	var intHash big.Int
	var hash [32]byte

	nonce := 0

	for nonce < math.MaxInt64 {
		//使用准备好的数据计算哈希值
		data := pow.InitData(nonce)
		hash = sha256.Sum256(data)

		fmt.Printf("\r%x", hash)
		intHash.SetBytes(hash[:])

		//判断哈希值是否落入目标域值中
		if intHash.Cmp(pow.Target) == -1 {
			break
		} else {
			nonce++
		}

	}
	fmt.Println()

	return nonce, hash[:]
}

// ToHex 将int64转变为[]byte
func ToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)

	}

	return buff.Bytes()
}

// Validate 判断包含nonce的区块的哈希是否在目标阈值内
func (pow *ProofOfWork) Validate() bool {
	var intHash big.Int

	// 构造挖矿结构体
	data := pow.InitData(pow.Block.Nonce)

	hash := sha256.Sum256(data)
	intHash.SetBytes(hash[:])

	// 判断哈希值是否落入目标域值
	return intHash.Cmp(pow.Target) == -1
}
