package blockchain

import (
	"Golang_Bitcoin_Sample/wallet"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
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

// CoinbaseTx 创建CoinBase交易
func CoinbaseTx(to, data string) *Transaction {
	if data == "" {
		randData := make([]byte, 24)
		_, err := rand.Read(randData)
		zap.L().Error("rand.Read() failed", zap.Error(err))
		data = fmt.Sprintf("%x", randData)
	}

	// Coinbase特征的输入结构
	txin := TxInput{[]byte{}, -1, nil, []byte(data)}
	//UTXO相关的输出结构
	txout := NewTXOutput(20, to)

	// 组装交易结构体
	tx := Transaction{nil, []TxInput{txin}, []TxOutput{*txout}}
	tx.ID = tx.Hash()

	return &tx
}

// 判断是不是Coinbase交易 IsCoinbaseTx
func (tx *Transaction) IsCoinbaseTx() bool {
	// 查看交易的输入结构特征
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].Out == -1
}

// NewTXOutput create a new TXOutput
func NewTXOutput(value int, address string) *TxOutput {
	txOut := &TxOutput{value, nil}
	txOut.SignTx([]byte(address))

	return txOut
}

func (out *TxOutput) SignTx(address []byte) {
	//从address反推公钥哈希（除去校验和）
	pubKeyHash := wallet.Base58Decode(address)

	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

	//输出结构--UTXO持有者公钥哈希赋值
	out.PubKeyHash = pubKeyHash
}
