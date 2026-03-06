package parser

import (
	"alpha-amm-engine/internal/dao/sqlmodel"
	"alpha-amm-engine/pkg/models"

	"github.com/ethereum/go-ethereum/core/types"
)

const (
	EventTypeMint    = "Mint"
	EventTypeBurn    = "Burn"
	EventTypeSwap    = "SWAP"
	EventTypeSync    = "SYNC"
	EventTypeCollect = "COLLECT" // V3 only
)

const (
	ParserTypeUniswapV2Pair = "uniswap_v2_pairs"
	ParserTypeUniswapV3     = "uniswap_v3"
)

// Parser 定义日志解析接口
type Parser interface {
	// Parse 解析日志为业务模型
	// 如果日志不符合该解析器关注的类型，返回 nil, nil
	Parse(log types.Log, blockTime int64, cfg *models.Blockchain) (*sqlmodel.AlphaPoolLiquidityEvent, error)

	// IsTargetTopic 检查是否是该解析器关注的事件 Topic
	IsTargetTopic(topic string) bool
}
