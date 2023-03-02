package wallet

import (
	"fmt"
	"os"
	"testing"
)

func TestWallet(t *testing.T) {
	// 创建钱包
	wallet := NewWallet()
	if wallet == nil {
		t.Error("NewWallet error: 创建钱包失败")
	}

	// 生成地址
	address := wallet.GenerateAddress()
	if len(address) == 0 {
		t.Error("GenerateAddress error: 地址生成失败")
	}

	// 校验地址
	isValid := ValidateAddress(string(address))
	if !isValid {
		t.Error("ValidateAddress error: 地址校验失败")
	}

	// 保存到文件并加载
	nodeId := "1"

	//初始化Wallets结构体
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)

	wallets.AddWallet()
	wallets.SaveFile(nodeId)
	loadedWallets, err := CreateWallets(nodeId)
	if err != nil {
		t.Errorf("CreateWallets error: %v", err)
	}

	if len(loadedWallets.Wallets) != 1 {
		t.Errorf("SaveFile error: 期望钱包数量为1，实际钱包数量为 %d", len(loadedWallets.Wallets))
	}

	// 删除测试文件
	walletFile := fmt.Sprintf(walletFile, nodeId)
	if _, err := os.Stat(walletFile); !os.IsNotExist(err) {
		err := os.Remove(walletFile)
		if err != nil {
			t.Errorf("删除测试文件失败: %v", err)
		}
	}

}
