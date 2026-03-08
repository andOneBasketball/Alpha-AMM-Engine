#!/bin/bash

solc --abi --bin -o ./build/uniswap_v2/ ./uniswap_v2/IUniswapV2ERC20.sol ./uniswap_v2/IUniswapV2Pair.sol --overwrite

abigen --bin=./build/uniswap_v2/IUniswapV2ERC20.bin --abi=./build/uniswap_v2/IUniswapV2ERC20.abi --pkg=IUniswapV2ERC20 --out=./build/uniswap_v2/IUniswapV2ERC20.go
abigen --bin=./build/uniswap_v2/IUniswapV2Pair.bin --abi=./build/uniswap_v2/IUniswapV2Pair.abi --pkg=IUniswapV2Pair --out=./build/uniswap_v2/IUniswapV2Pair.go

solc --abi --bin -o ./build/uniswap_v3/ ./uniswap_v3/IUniswapV3Pool.sol --overwrite
abigen --bin=./build/uniswap_v3/IUniswapV3Pool.bin --abi=./build/uniswap_v3/IUniswapV3Pool.abi --pkg=IUniswapV3Pool --out=./build/uniswap_v3/IUniswapV3Pool.go