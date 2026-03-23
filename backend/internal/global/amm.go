package global

import (
	"math/big"

	"github.com/shopspring/decimal"
)

const (
	UniswapV2FactoryAddress   = "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"
	UniswapV2PairInitCodeHash = "96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f"

	UniswapV3FactoryAddress   = "0x1F98431c8aD98523631AE4a59f267346ea31F984"
	UniswapV3PoolInitCodeHash = "e34f199b19b2b4f47f68442619d555527d244f78a3297ea89325f843f87b8b54"
)

const (
	PoolTypeUniswapV2 = "Uniswap V2"
	PoolTypeUniswapV3 = "Uniswap V3"
)

var Q96 = new(big.Int).Lsh(big.NewInt(1), 96)
var Q96D = decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 96), 0)
