package handler

import (
	"alpha-amm-engine/internal/global"
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// computePairAddress 通过 CREATE2 计算 Uniswap V2 pair 地址，并返回是否与用户传入顺序相反
//
// Uniswap V2 pair 地址公式：
//
//	keccak256(0xff ++ factory ++ keccak256(token0 ++ token1) ++ initCodeHash)[12:]
//	其中 token0 < token1（地址数值升序）
func UniswapV2PairFor(token0, token1 string) (pairAddr common.Address, isSwapped bool) {
	isSwapped = false

	// 1. Uniswap V2 Factory 地址 (以太坊主网)
	factoryAddr := common.HexToAddress(global.UniswapV2FactoryAddress)

	// 2. Token0 和 Token1 (注意：计算前必须按数值大小排序)
	// 比如 DAI 和 WETH
	token0Addr := common.HexToAddress(token0)
	token1Addr := common.HexToAddress(token1)

	if bytes.Compare(token0Addr.Bytes(), token1Addr.Bytes()) > 0 {
		token0Addr, token1Addr = token1Addr, token0Addr
		isSwapped = true
	}

	// 3. 计算 Salt: keccak256(abi.encodePacked(token0, token1))
	// 注意：在 Solidity 中排序逻辑是 token0 < token1
	salt := crypto.Keccak256(token0Addr.Bytes(), token1Addr.Bytes())

	// 4. Init Code Hash: type(UniswapV2Pair).creationCode 的哈希
	// Uniswap V2 固定的 Pair Init Code Hash
	initCodeHash := common.Hex2Bytes(global.UniswapV2PairInitCodeHash)

	// 5. 执行 CREATE2 地址计算
	// [0xff] + [factory] + [salt] + [initCodeHash]
	input := append([]byte{0xff}, factoryAddr.Bytes()...)
	input = append(input, salt...)
	input = append(input, initCodeHash...)

	pairAddrHash := crypto.Keccak256(input)
	pairAddr = common.BytesToAddress(pairAddrHash[12:]) // 取后 20 字节

	return
}
