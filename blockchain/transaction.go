package blockchain

import (
	"Golang_Bitcoin_Sample/wallet"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"go.uber.org/zap"
	"log"
	"math/big"
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

// 输出结构数组
type TxOutputs struct {
	Outputs []TxOutput
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
		return nil
	}

	return encoded.Bytes()
}

// SerializeOutputs 序列化输出结构切片
func (outs TxOutputs) SerializeOutputs() []byte {
	var buffer bytes.Buffer

	encode := gob.NewEncoder(&buffer)
	err := encode.Encode(outs)
	zap.L().Error("enc.Encode() failed", zap.Error(err))

	return buffer.Bytes()
}

// DeserializeOutputs 反序列化输出结构切片
func DeserializeOutputs(data []byte) TxOutputs {
	var outputs TxOutputs

	decode := gob.NewDecoder(bytes.NewReader(data))
	err := decode.Decode(&outputs)
	zap.L().Error("enc.Encode() failed", zap.Error(err))

	return outputs
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

// NewTransaction 创建新交易
func NewTransaction(w *wallet.Wallet, to string, amount int, UTXO *UTXOSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	// 获取交易发送方公钥哈希
	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)
	// 获取花销总额以及涉及的UTXO
	accumulate, validOutputs := UTXO.FindSpendableOutputs(pubKeyHash, amount)

	if accumulate < amount {
		log.Panic("Error: not enough funds")
	}

	// 将涉及的UTXO用于构造输入结构
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		zap.L().Error("hex.DecodeString() failed", zap.Error(err))

		// out是一笔输出结构中的交易排名次序（从0开始）
		for _, out := range outs {
			input := TxInput{txID, out, nil, w.PublicKey}
			inputs = append(inputs, input)
		}
	}

	from := fmt.Sprintf("%s", w.GenerateAddress())

	// 构造输出结构（UTXO不能拆分，多余的金额通过一笔新的UTXO发回给自己）
	outputs = append(outputs, *NewTXOutput(amount, to))
	if accumulate > amount {
		outputs = append(outputs, *NewTXOutput(accumulate-amount, from))
	}

	// 组装交易结构体
	tx := Transaction{nil, inputs, outputs}
	tx.ID = tx.Hash()

	// 对交易进行签名，将签名信息保存在输入结构中
	UTXO.Blockchain.SignTransaction(&tx, w.PrivateKey)

	return &tx
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
	txOut.GetPublicKeyHash([]byte(address))

	return txOut
}

func (out *TxOutput) GetPublicKeyHash(address []byte) {
	//从address反推公钥哈希（除去校验和）
	pubKeyHash := wallet.Base58Decode(address)

	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

	//输出结构--UTXO持有者公钥哈希赋值
	out.PubKeyHash = pubKeyHash
}

// PubKeyHashEquals 判断公钥哈希是否相等
func (out *TxOutput) PubKeyHashEquals(pubKeyHash []byte) bool {
	return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}

// Sign 对交易进行签署，并将签名保存在输入结构中
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	// 币基交易没有输入结构
	if tx.IsCoinbaseTx() {
		return
	}

	for _, in := range tx.Inputs {
		// 判断map中的Id索引的是否索引对应的交易
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}

	// 获取精简后的交易
	txCopy := tx.TrimmedCopy()

	// 遍历交易中的所有输入结构
	for inId, in := range txCopy.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inId].Signature = nil

		//输入脚本中的公钥，引用的UTXO持有者的公钥哈希作代替
		//因为输入输出脚本合并执行的过程中，比对的也是公钥哈希后的结果，这里简化了公钥计算哈希值的过程
		txCopy.Inputs[inId].PubKey = prevTX.Outputs[in.Out].PubKeyHash

		dataToSign := fmt.Sprintf("%x\n", txCopy)

		// 使用私钥对交易输入信息进行签名
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, []byte(dataToSign))
		zap.L().Error("ecdsa.Sign failed()", zap.Error(err))

		// r、s为椭圆曲线中的坐标信息
		signature := append(r.Bytes(), s.Bytes()...)

		//对交易原本Tx的签名域进行赋值
		tx.Inputs[inId].Signature = signature
		txCopy.Inputs[inId].PubKey = nil
	}
}

// Verify 验证交易是否合法
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	//币基交易不需要验证UTXO的引用
	if tx.IsCoinbaseTx() {
		return true
	}

	// 判断map中的Id索引的是否索引对应的交易
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("Previous transaction not correct")
		}
	}

	// 获得精简后的交易，以及用于签名验证的椭圆曲线模型
	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()

	for inId, in := range tx.Inputs {
		// 交易原数据获取
		prevTx := prevTXs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inId].Signature = nil
		txCopy.Inputs[inId].PubKey = prevTx.Outputs[in.Out].PubKeyHash

		dataToVerify := fmt.Sprintf("%x\n", txCopy)

		// 签名数据坐标化
		r := big.Int{}
		s := big.Int{}

		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])

		// 公钥数据坐标化
		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])
		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}

		// 验证交易是否合法
		if ecdsa.Verify(&rawPubKey, []byte(dataToVerify), &r, &s) == false {
			return false
		}
		txCopy.Inputs[inId].PubKey = nil
	}

	return true
}

// TrimmedCopy 获取除了输入结构的交易摘要，用于签名
func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	// 输出结构剔除签名和公钥信息
	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{in.ID, in.Out, nil, nil})
	}

	// 获取完整的输出结构
	for _, out := range tx.Outputs {
		outputs = append(outputs, TxOutput{out.Value, out.PubKeyHash})
	}

	txCopy := Transaction{tx.ID, inputs, outputs}

	return txCopy
}
