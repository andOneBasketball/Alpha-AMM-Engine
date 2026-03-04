#!/bin/bash

solc --abi --bin -o ./build/ ./IUniswapV2ERC20.sol ./IUniswapV2Pair.sol --overwrite

abigen --bin=./build/IUniswapV2ERC20.bin --abi=./build/IUniswapV2ERC20.abi --pkg=IUniswapV2ERC20 --out=./build/IUniswapV2ERC20.go
abigen --bin=./build/IUniswapV2Pair.bin --abi=./build/IUniswapV2Pair.abi --pkg=IUniswapV2Pair --out=./build/IUniswapV2Pair.go