package blockchain

import (
	"crypto/sha256"
	"go.uber.org/zap"
)

type MerkleTree struct {
	MerkleRoot *MerkleNode
}

type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

// NewMerkleNode 构建新的MerkleTree Node
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := MerkleNode{}

	//如果是叶节点
	if left == nil && right == nil {
		hash := sha256.Sum256(data)
		node.Data = hash[:]
	} else { //如果是中间节点
		prevHashes := append(left.Data, right.Data...)
		hash := sha256.Sum256(prevHashes)
		node.Data = hash[:]
	}

	node.Left = left
	node.Right = right

	return &node
}

// NewMerkleTree 构建MerkleTree 获得merkleRoot
func NewMerkleTree(data [][]byte) *MerkleTree {
	var nodes []MerkleNode

	//将所有交易构造成MerkleTree Node
	for _, dat := range data {
		node := NewMerkleNode(nil, nil, dat)
		nodes = append(nodes, *node)
	}

	if len(nodes) == 0 {
		zap.L().Error("There is no transaction")
	}

	// 由下往上计算MerkleTree，直至根节点
	for len(nodes) > 1 {
		//节点数量非偶数时，复制最后一个节点
		if len(nodes)%2 != 0 {
			nodes = append(nodes, nodes[len(nodes)-1])
		}

		// 将本层的MerkleTree Node两两构筑成中间节点
		var level []MerkleNode
		for i := 0; i < len(nodes); i += 2 {
			node := NewMerkleNode(&nodes[i], &nodes[i+1], nil)
			level = append(level, *node)
		}

		nodes = level
	}

	//切片的第一个节点为merkleRoot
	tree := MerkleTree{&nodes[0]}

	return &tree
}
