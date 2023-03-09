package blockchain

//区块结构
//Transactions、MerkleRoot、PrevBlockHash、Nonce、Height需要进行哈希计算
type Block struct {
	Timestamp     int64  //时间戳
	Hash          []byte //当前区块哈希值
	Transactions  []*Transaction
	MerkleRoot    []byte //默克尔树根
	PrevBlockHash []byte //前块哈希值
	Nonce         int    //随机值
	Height        int    //区块高度
}
