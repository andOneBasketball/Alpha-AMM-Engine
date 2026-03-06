package scan

import (
	"context"
	"strings"
	"sync"
	"time"

	"alpha-amm-engine/internal/dao"
	"alpha-amm-engine/internal/dao/sqlmodel"
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/pkg/models"
	"alpha-amm-engine/svc/scan/parser"

	"github.com/duke-git/lancet/v2/maputil"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

type Runner struct {
	config  *models.Blockchain
	client  *EthClient
	parsers map[string]parser.Parser
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewRunner(cfg *models.Blockchain) (*Runner, error) {
	client, err := NewEthClient(cfg.RPC)
	if err != nil {
		return nil, err
	}

	// 初始化解析器
	v2Parser, err := parser.NewUniswapV2Parser()
	if err != nil {
		return nil, err
	}
	parsers := map[string]parser.Parser{
		parser.ParserTypeUniswapV2Pair: v2Parser,
	}

	return &Runner{
		config:  cfg,
		client:  client,
		parsers: parsers,
		stopCh:  make(chan struct{}),
	}, nil
}

func (r *Runner) Start(ctx context.Context) {
	logger.Log.Info("Starting scanner runner...")
	r.wg.Add(1)
	go r.runLoop(ctx)
}

func (r *Runner) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	r.client.Close()
	logger.Log.Info("Scanner runner stopped")
}

func (r *Runner) runLoop(ctx context.Context) {
	defer r.wg.Done()

	isCatchingUp := true
	ticker := time.NewTimer(time.Duration(0))
	defer ticker.Stop()

	// 确定起始区块
	currentBlock := r.config.StartBlock

	// 获取最后扫描区块
	lastEvent, err := dao.AlphaPoolLiquidityEvent.WithContext(ctx).Order(dao.AlphaPoolLiquidityEvent.BlockNumber.Desc()).First()
	if err == nil && lastEvent.BlockNumber > currentBlock {
		currentBlock = lastEvent.BlockNumber + 1
	}

	for {
		select {
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			delay := time.Duration(r.config.SyncInterval) * time.Second
			if isCatchingUp {
				delay = time.Duration(r.config.CatchUpInterval) * time.Second
			}
			ticker.Reset(delay)

			// 获取最新区块高度
			latestBlock, err := r.client.GetBlockNumber(ctx)
			if err != nil {
				logger.Log.Error("Failed to get latest block number", zap.Error(err))
				continue
			}

			// 确认区块确认数
			safeBlock := latestBlock - r.config.MinQueryRange
			if currentBlock > safeBlock {
				isCatchingUp = false
				continue
			}

			// 计算本次扫描的结束区块
			endBlock := currentBlock + r.config.MaxQueryRange
			if endBlock > latestBlock {
				endBlock = latestBlock
				isCatchingUp = false
			}

			logger.Log.Info("Scanning block range",
				zap.Int64("from", currentBlock),
				zap.Int64("to", endBlock),
				zap.Bool("isCatchingUp", isCatchingUp))

			if err := r.scanRange(ctx, currentBlock, endBlock); err != nil {
				logger.Log.Error("Failed to scan range",
					zap.Int64("from", currentBlock),
					zap.Int64("to", endBlock),
					zap.Error(err))
				// 失败重试或跳过交给外层策略，这里简单处理为下一轮重试
				continue
			}

			// 更新下一次扫描的起始位置
			currentBlock = endBlock + 1
		}
	}
}

func (r *Runner) scanRange(ctx context.Context, from, to int64) error {
	// 1. 获取日志
	if len(r.config.Contracts) == 0 {
		logger.Log.Warn("No contracts configured for scanning")
		return nil
	}
	contracts := maputil.Keys(r.config.Contracts)

	logs, err := r.client.GetLogs(ctx, from, to, contracts)
	if err != nil {
		return err
	}

	if len(logs) == 0 {
		return nil
	}

	// 2. 预取区块时间 (简单优化：只取范围内的几个关键区块时间，或者对每个Log单独取)
	// 为了简化，这里我们暂时对每个Log所属区块去获取时间，或者缓存BlockTime
	// 生产环境建议：批量获取BlockHeader或缓存
	blockTimeCache := make(map[int64]int64)

	var eventsToSave []*sqlmodel.AlphaPoolLiquidityEvent // 使用 sqlmodel 类型

	for _, log := range logs {
		// 获取区块时间
		bTime, ok := blockTimeCache[int64(log.BlockNumber)]
		if !ok {
			t, err := r.client.GetBlockTime(ctx, int64(log.BlockNumber))
			if err != nil {
				logger.Log.Error("Failed to get block time", zap.Error(err))
				continue
			}
			bTime = t
			blockTimeCache[int64(log.BlockNumber)] = bTime
		}

		// 遍历解析器
		parserType, exists := r.config.Contracts[log.Address.Hex()]
		if !exists {
			continue
		}
		parserType = strings.TrimSpace(parserType[0:24]) // 取前24字符，去除空格
		p, ok := r.parsers[parserType]
		if !ok {
			logger.Log.Warn("No parser found for type", zap.String("type", parserType))
			continue
		}

		logger.Log.Info("Processing log", zap.String("address", log.Address.Hex()), zap.String("topic", log.Topics[0].Hex()))
		if len(log.Topics) > 0 && p.IsTargetTopic(log.Topics[0].Hex()) {
			event, err := p.Parse(log, bTime, r.config)
			if err != nil {
				logger.Log.Error("Failed to parse log", zap.Error(err))
				continue
			}
			if event != nil {
				eventsToSave = append(eventsToSave, event)
			}
		}
	}

	// 3. 批量入库
	if len(eventsToSave) > 0 {
		return dao.AlphaPoolLiquidityEvent.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tx_hash"}, {Name: "log_index"}},
			DoNothing: true,
		}).CreateInBatches(eventsToSave, 100)
	}
	return nil
}
