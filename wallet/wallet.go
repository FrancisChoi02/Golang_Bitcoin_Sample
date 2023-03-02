package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/ripemd160"
	"io/ioutil"
	"log"
	"os"
)

//常量定义
const (
	ChecksumLen = 4
	version     = byte(0x00)
)

// 钱包信息文件
const walletFile = "./tmp/wallets_%s.dat"

// 钱包结构
type Wallet struct {
	PrivateKey ecdsa.PrivateKey //私钥
	PublicKey  []byte           //公钥
}

// 地址与钱包结构的映射
type Wallets struct {
	Wallets map[string]*Wallet
}

// 创建钱包
func NewWallet() *Wallet {
	// 1. 随机生成秘钥对
	private, public := NewKeyPair()

	// 2. 初始化新钱包
	wallet := Wallet{private, public}

	return &wallet
}

// NewKeyPair 密钥对生成
func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	//获得椭圆曲线
	curve := elliptic.P256()

	//通过曲线与随机数 生成私钥
	privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	//利用私钥推导出公钥
	publicKey := append(privateKey.PublicKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...) //语法糖，将多个参数合并成一个切片

	return *privateKey, publicKey
}

// 计算公钥
func PublicKeyHash(pubKey []byte) []byte {
	// 1. 进行一次SHA-256哈希计算
	publicHash := sha256.Sum256(pubKey)

	// 2. 进行一次Ripemd160计算
	RIPEMD160Hasher := ripemd160.New()
	_, err := RIPEMD160Hasher.Write(publicHash[:])
	if err != nil {
		log.Panic(err)
	}

	// 3.返回标准的公钥哈希
	publicRIPEMD160 := RIPEMD160Hasher.Sum(nil)

	return publicRIPEMD160
}

// GenerateAddress 由公钥生成地址
func (w Wallet) GenerateAddress() []byte {
	// 1. 获得公钥哈希
	pubHash := PublicKeyHash(w.PublicKey)

	// 2. 组装版本号
	versionedHash := append([]byte{version}, pubHash...)
	// 3. 获得校验和
	checksum := Checksum(versionedHash)

	// 4. 组装校验和，并进行58编码
	fullHash := append(versionedHash, checksum...)
	address := Base58Encode(fullHash)

	return address
}

// Checksum 获取校验和 输入为0x00+公钥hash
func Checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])

	//摘取第二次哈希结果的 前4个字节位 为校验和
	return secondSHA[:ChecksumLen]
}

// ValidateAddress 验证地址合法性
func ValidateAddress(address string) bool {
	// 1. Base58解码
	pubKeyHash := Base58Decode([]byte(address))
	length := len(pubKeyHash)

	// 2. 提取实际的的校验码
	actualChecksum := pubKeyHash[length-ChecksumLen:]

	// 3. 组装参数获得目标校验码
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : length-ChecksumLen]

	targetChecksum := Checksum(append([]byte{version}, pubKeyHash...)) //语法糖，将多个参数合并成一个切片

	// 4. 比对并返回校验码
	return bytes.Compare(actualChecksum, targetChecksum) == 0
}

//Base58Encode Base58编码
func Base58Encode(input []byte) []byte {
	encode := base58.Encode(input)

	return []byte(encode)
}

//Base58Decode Base58解码
func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input[:]))
	if err != nil {
		log.Panic(err)
	}

	return decode
}

//SaveFile 保存钱包信息到文件
func (ws *Wallets) SaveFile(nodeId string) {
	// 1.获取节点Id对应的钱包文件
	var content bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeId) //根据不同节点号进行钱包存储

	// 2.获取椭圆曲线对象并进行加密，保证私钥文件的存储安全
	gob.Register(elliptic.P256())

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	if err != nil {
		fmt.Println("ws is ")
	}

	// 3.将加密后的数据写入文件中
	err = ioutil.WriteFile(walletFile, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err)
	}
}

//LoadFile 从加载文件夹加载钱包信息
func (ws *Wallets) LoadFile(nodeId string) error {
	// 1.获取节点Id对应的钱包文件
	walletFile := fmt.Sprintf(walletFile, nodeId)
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err
	}

	// 2.打开钱包文件后，读取文件中的数据
	var wallets Wallets

	fileContent, err := ioutil.ReadFile(walletFile)
	if err != nil {
		return err
	}

	// 3.获取椭圆曲线对象并进行解密
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		return err
	}

	ws.Wallets = wallets.Wallets

	return nil
}

// CreateWallets 从文件中加载所有钱包信息
func CreateWallets(nodeId string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	err := wallets.LoadFile(nodeId)

	return &wallets, err
}

//AddWallet 添加钱包信息
func (ws *Wallets) AddWallet() string {
	// 1. 初始化钱包对象
	wallet := NewWallet()
	address := fmt.Sprintf("%s", wallet.GenerateAddress())

	// 2. 映射地址与钱包结构体的关系
	ws.Wallets[address] = wallet

	return address
}

//GetAllAddresses 获取所有钱包信息
func (ws *Wallets) GetAllAddresses() []string {
	var addresses []string

	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses
}

// 根据地址获取钱包
func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}
