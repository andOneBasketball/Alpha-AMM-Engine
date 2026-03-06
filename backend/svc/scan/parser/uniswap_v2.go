package parser

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"alpha-amm-engine/internal/contracts"
	"alpha-amm-engine/internal/dao/sqlmodel"
	"alpha-amm-engine/pkg/models"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// UniswapV2Parser 解析 Uniswap V2 事件
type UniswapV2Parser struct {
	contractABI abi.ABI
}

func NewUniswapV2Parser() (*UniswapV2Parser, error) {
	parsedABI, err := abi.JSON(strings.NewReader(contracts.IUniswapV2PairMetaData.ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse V2 Pair ABI: %w", err)
	}
	return &UniswapV2Parser{
		contractABI: parsedABI,
	}, nil
}

func (p *UniswapV2Parser) IsTargetTopic(topic string) bool {
	// Mint: 0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fefdf639458c9f77b96
	// Burn: 0xdccd412f0b1252819cb1fd330b93224ca42612892bb3f4f789976e6d81936496
	mintTopic := p.contractABI.Events["Mint"].ID.Hex()
	burnTopic := p.contractABI.Events["Burn"].ID.Hex()
	return topic == mintTopic || topic == burnTopic
}

func (p *UniswapV2Parser) Parse(log types.Log, blockTime int64, cfg *models.Blockchain) (*sqlmodel.AlphaPoolLiquidityEvent, error) {
	if len(log.Topics) == 0 {
		return nil, nil
	}
	topic := log.Topics[0].Hex()

	var eventName string
	var amount0, amount1 *big.Int
	var provider common.Address

	// V2 Mint: event Mint(address indexed sender, uint amount0, uint amount1);
	// V2 Burn: event Burn(address indexed sender, uint amount0, uint amount1, address indexed to);

	switch topic {
	case p.contractABI.Events[EventTypeMint].ID.Hex():
		eventName = EventTypeMint
		event := struct {
			Sender  common.Address
			Amount0 *big.Int
			Amount1 *big.Int
		}{}
		// Unpack 非 indexed 字段
		if err := p.contractABI.UnpackIntoInterface(&event, EventTypeMint, log.Data); err != nil {
			return nil, err
		}
		// V2 Mint sender is indexed at topic[1]
		if len(log.Topics) > 1 {
			provider = common.BytesToAddress(log.Topics[1].Bytes())
		}
		amount0 = event.Amount0
		amount1 = event.Amount1

	case p.contractABI.Events[EventTypeBurn].ID.Hex():
		eventName = EventTypeBurn
		event := struct {
			Sender  common.Address
			Amount0 *big.Int
			Amount1 *big.Int
			To      common.Address
		}{}
		// Burn event: sender is indexed (topic[1]), to is indexed (topic[2])
		// data contains amount0, amount1
		if err := p.contractABI.UnpackIntoInterface(&event, EventTypeBurn, log.Data); err != nil {
			return nil, err
		}
		// Burn通常sender是router，to是接收者，这里我们可以记录sender作为触发者
		if len(log.Topics) > 1 {
			provider = common.BytesToAddress(log.Topics[1].Bytes())
		}
		amount0 = event.Amount0
		amount1 = event.Amount1

	default:
		return nil, nil
	}

	// 构造存储模型
	model := &sqlmodel.AlphaPoolLiquidityEvent{
		ChainID:        int32(cfg.ChainID),
		PoolAddress:    log.Address.Hex(),
		TxHash:         log.TxHash.Hex(),
		LogIndex:       int32(log.Index),
		EventName:      eventName,
		Provider:       provider.Hex(),
		Amount0:        amount0.String(),
		Amount1:        amount1.String(),
		BlockNumber:    int64(log.BlockNumber),
		BlockTimestamp: time.Unix(blockTime, 0),
		// BlockTimestamp 需要在外部获取Block后传入
		// CreatedAt 由DB自动处理
	}

	return model, nil
}
