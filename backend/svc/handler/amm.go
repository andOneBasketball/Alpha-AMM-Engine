package handler

import (
	"alpha-amm-engine/internal/global"
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// UniswapV2PairFor 通过 CREATE2 计算 Uniswap V2 pair 地址，并返回是否与用户传入顺序相反
//
// keccak256(0xff ++ factory ++ keccak256(token0 ++ token1) ++ initCodeHash)[12:]
// 其中 token0 < token1（地址数值升序）
func UniswapV2PairFor(token0, token1 string) (pairAddr common.Address, isSwapped bool) {
	factoryAddr := common.HexToAddress(global.UniswapV2FactoryAddress)
	token0Addr := common.HexToAddress(token0)
	token1Addr := common.HexToAddress(token1)

	if bytes.Compare(token0Addr.Bytes(), token1Addr.Bytes()) > 0 {
		token0Addr, token1Addr = token1Addr, token0Addr
		isSwapped = true
	}

	salt := crypto.Keccak256(token0Addr.Bytes(), token1Addr.Bytes())
	initCodeHash := common.Hex2Bytes(global.UniswapV2PairInitCodeHash)

	input := append([]byte{0xff}, factoryAddr.Bytes()...)
	input = append(input, salt...)
	input = append(input, initCodeHash...)

	pairAddr = common.BytesToAddress(crypto.Keccak256(input)[12:])
	return
}

// UniswapV3PoolFor 通过 CREATE2 计算 Uniswap V3 pool 地址，并返回 token 是否与用户传入顺序相反
//
// salt = keccak256(abi.encode(token0, token1, fee))  — 每个参数 ABI 补零至 32 字节
// keccak256(0xff ++ factory ++ salt ++ initCodeHash)[12:]
func UniswapV3PoolFor(token0, token1 string, fee int64) (poolAddr common.Address, isSwapped bool) {
	factoryAddr := common.HexToAddress(global.UniswapV3FactoryAddress)
	token0Addr := common.HexToAddress(token0)
	token1Addr := common.HexToAddress(token1)

	if bytes.Compare(token0Addr.Bytes(), token1Addr.Bytes()) > 0 {
		token0Addr, token1Addr = token1Addr, token0Addr
		isSwapped = true
	}

	// abi.encode(address token0, address token1, uint24 fee)
	// 每个字段占 32 字节，address 左补零，uint24 右对齐左补零
	saltInput := make([]byte, 96)
	copy(saltInput[12:32], token0Addr.Bytes()) // bytes [0..11] 为零，[12..31] 为地址
	copy(saltInput[44:64], token1Addr.Bytes()) // bytes [32..43] 为零，[44..63] 为地址
	saltInput[93] = byte(fee >> 16)            // bytes [64..92] 为零，[93..95] 为 fee（uint24）
	saltInput[94] = byte(fee >> 8)
	saltInput[95] = byte(fee)

	salt := crypto.Keccak256(saltInput)
	initCodeHash := common.Hex2Bytes(global.UniswapV3PoolInitCodeHash)

	input := append([]byte{0xff}, factoryAddr.Bytes()...)
	input = append(input, salt...)
	input = append(input, initCodeHash...)

	poolAddr = common.BytesToAddress(crypto.Keccak256(input)[12:])
	return
}
