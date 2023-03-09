package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"go.uber.org/zap"
)

type Transaction struct {
	ID      []byte     // 交易ID
	Inputs  []TxInput  // 输入结构
	Outputs []TxOutput // 输出结构
}

// 输入结构
type TxInput struct {
	ID        []byte // 交易哈希
	Out       int    // 输出索引
	Signature []byte // 签名
	PubKey    []byte // 签署人的公钥
}

// 输出结构
type TxOutput struct {
	Value      int    // 输出金额
	PubKeyHash []byte // UTXO持有者的公钥哈希
}

// Hash 交易ID获取
func (tx *Transaction) Hash() []byte {
	tmpTx := *tx
	tmpTx.ID = []byte{}

	//将整个tx进行哈希计算，结果即为tx的ID
	hash := sha256.Sum256(tmpTx.Serialize())

	return hash[:]
}

// Serialize 交易数据序列化
func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	//初始化编码器
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		zap.L().Error("enc.Encode() failed", zap.Error(err))
	}

	return encoded.Bytes()
}

// DeserializeTransaction 反序列化数据获得交易
func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction

	//初始化解码器
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	if err != nil {
		zap.L().Error("enc.Decode() failed", zap.Error(err))
	}
	return transaction
}
