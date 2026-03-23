package poolsync

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"alpha-amm-engine/internal/contracts"
	"alpha-amm-engine/internal/dao"
	"alpha-amm-engine/internal/dao/sqlmodel"
	"alpha-amm-engine/internal/global"
	"alpha-amm-engine/pkg/database"
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/svc/handler"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	minTick = -887272
	maxTick = 887272
)

type PoolSyncer struct {
	ethClient *ethclient.Client
	chainID   int32
	limiter   *rate.Limiter
	TokenInfo map[string]*sqlmodel.AlphaToken // token_address => token_info
}

func NewPoolSyncer(rpcURL string, chainID int32, rpcRatePerSecond int) (*PoolSyncer, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	return &PoolSyncer{
		ethClient: client,
		chainID:   chainID,
		limiter:   rate.NewLimiter(rate.Limit(rpcRatePerSecond), rpcRatePerSecond),
		TokenInfo: make(map[string]*sqlmodel.AlphaToken),
	}, nil
}

func (s *PoolSyncer) Close() {
	s.ethClient.Close()
}

// SyncAll 遍历 config_pair 表，同步每个配置对的池状态到 alpha_amm_pool
func (s *PoolSyncer) SyncAll(ctx context.Context) {
	pairs, err := dao.ConfigPair.WithContext(ctx).Find()
	if err != nil {
		logger.Log.Error("Failed to query config_pair", zap.Error(err))
		return
	}

	var tokenList []*sqlmodel.AlphaToken
	if err := dao.AlphaToken.WithContext(ctx).Scan(&tokenList); err != nil {
		logger.Log.Warn("Failed to load token info from database", zap.Error(err))
	}
	for _, token := range tokenList {
		s.TokenInfo[strings.ToLower(token.Address)] = token
	}

	blockNum, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		logger.Log.Error("Failed to get block number", zap.Error(err))
		return
	}

	logger.Log.Info("Starting pool sync", zap.Int("pairs", len(pairs)), zap.Uint64("block", blockNum))

	for _, pair := range pairs {
		if err := s.syncPair(ctx, pair, int64(blockNum)); err != nil {
			logger.Log.Error("Failed to sync pair",
				zap.Int64("config_pair_id", pair.ID),
				zap.String("token0", pair.Token0Address),
				zap.String("token1", pair.Token1Address),
				zap.String("pool_type", pair.PoolType),
				zap.Error(err))
		}
	}

	logger.Log.Info("Pool sync completed", zap.Int("pairs", len(pairs)))
}

func (s *PoolSyncer) syncPair(ctx context.Context, pair *sqlmodel.ConfigPair, blockNum int64) error {
	switch pair.PoolType {
	case "Uniswap V2":
		return s.syncV2Pair(ctx, pair, blockNum)
	case "Uniswap V3":
		return s.syncV3Pool(ctx, pair, blockNum)
	default:
		logger.Log.Warn("Unknown pool type, skipping",
			zap.Int64("id", pair.ID),
			zap.String("pool_type", pair.PoolType))
		return nil
	}
}

func (s *PoolSyncer) syncV2Pair(ctx context.Context, pair *sqlmodel.ConfigPair, blockNum int64) error {
	pairAddr, isSwapped := handler.UniswapV2PairFor(pair.Token0Address, pair.Token1Address)

	contract, err := contracts.NewIUniswapV2Pair(pairAddr, s.ethClient)
	if err != nil {
		return err
	}

	reserves, err := contract.GetReserves(&bind.CallOpts{Context: ctx})
	if err != nil {
		return err
	}

	// 以合约内部规范顺序（地址升序）存储 token0/token1
	token0, token1 := pair.Token0Address, pair.Token1Address
	if isSwapped {
		token0, token1 = token1, token0
	}

	token0Info, _ := s.TokenInfo[strings.ToLower(token0)]
	token1Info, _ := s.TokenInfo[strings.ToLower(token1)]

	r0 := decimal.NewFromBigInt(reserves.Reserve0, 0).Div(decimal.New(1, token0Info.Decimals))
	r1 := decimal.NewFromBigInt(reserves.Reserve1, 0).Div(decimal.New(1, token1Info.Decimals))
	price0, price1 := "0", "0"
	if r0.IsPositive() {
		price0 = r1.Div(r0).String()
	}
	if r1.IsPositive() {
		price1 = r0.Div(r1).String()
	}

	logger.Log.Info("Fetched Uniswap V2 pair data",
		zap.String("pair", pairAddr.Hex()),
		zap.Any("token0_info", token0Info),
		zap.Any("token1_info", token1Info),
		zap.String("reserve0", r0.String()),
		zap.String("reserve1", r1.String()),
		zap.String("price0", price0),
		zap.String("price1", price1),
	)
	return s.upsertPool(ctx, &sqlmodel.AlphaAmmPool{
		ChainID:         s.chainID,
		PoolAddress:     strings.ToLower(pairAddr.Hex()),
		Token0Address:   strings.ToLower(token0),
		Token1Address:   strings.ToLower(token1),
		PoolType:        "Uniswap V2",
		FeeTier:         pair.FeeTier,
		Reserve0:        reserves.Reserve0.String(),
		Reserve1:        reserves.Reserve1.String(),
		Price0:          price0,
		Price1:          price1,
		LastBlockNumber: blockNum,
		LastUpdatedAt:   time.Now(),
	})
}

func (s *PoolSyncer) syncV3Pool(ctx context.Context, pair *sqlmodel.ConfigPair, blockNum int64) error {
	poolAddr, isSwapped := handler.UniswapV3PoolFor(pair.Token0Address, pair.Token1Address, int64(pair.FeeTier))
	addrStr := strings.ToLower(poolAddr.Hex())

	contract, err := contracts.NewIUniswapV3Pool(poolAddr, s.ethClient)
	if err != nil {
		return err
	}

	callOpts := &bind.CallOpts{Context: ctx}

	slot0, err := contract.Slot0(callOpts)
	if err != nil {
		return err
	}

	liquidity, err := contract.Liquidity(callOpts)
	if err != nil {
		return err
	}

	token0, token1 := pair.Token0Address, pair.Token1Address
	if isSwapped {
		token0, token1 = token1, token0
	}

	token0Info, _ := s.TokenInfo[strings.ToLower(token0)]
	token1Info, _ := s.TokenInfo[strings.ToLower(token1)]

	sqrtPriceD := decimal.NewFromBigInt(slot0.SqrtPriceX96, 0)
	v3Price0 := sqrtPriceD.Div(global.Q96D).Pow(decimal.NewFromInt(2)).Mul(decimal.New(1, token0Info.Decimals-token1Info.Decimals))
	v3Price1 := decimal.NewFromInt(1).Div(v3Price0)

	logger.Log.Info("Fetched Uniswap V3 pool data",
		zap.String("pool", addrStr),
		zap.Any("token0_info", token0Info),
		zap.Any("token1_info", token1Info),
		zap.Int32("fee_tier", pair.FeeTier),
		zap.Int32("tick_space", pair.TickSpace),
		zap.String("liquidity", liquidity.String()),
		zap.String("sqrt_price_x96", slot0.SqrtPriceX96.String()),
		zap.Int32("current_tick", int32(slot0.Tick.Int64())),
		zap.String("price0", v3Price0.String()),
		zap.String("price1", v3Price1.String()),
	)

	if err := s.upsertPool(ctx, &sqlmodel.AlphaAmmPool{
		ChainID:         s.chainID,
		PoolAddress:     addrStr,
		Token0Address:   strings.ToLower(token0),
		Token1Address:   strings.ToLower(token1),
		PoolType:        "Uniswap V3",
		FeeTier:         pair.FeeTier,
		TickSpace:       pair.TickSpace,
		Price0:          v3Price0.String(),
		Price1:          v3Price1.String(),
		Liquidity:       liquidity.String(),
		SqrtPriceX96:    slot0.SqrtPriceX96.String(),
		CurrentTick:     int32(slot0.Tick.Int64()),
		LastBlockNumber: blockNum,
		LastUpdatedAt:   time.Now(),
	}); err != nil {
		return err
	}

	// 同步 tickmap 中所有已初始化 tick 的数据
	return s.syncV3Ticks(ctx, contract, addrStr, pair.TickSpace)
}

// syncV3Ticks 遍历 tickBitmap，获取每个已初始化 tick 的流动性和手续费数据
func (s *PoolSyncer) syncV3Ticks(ctx context.Context, contract *contracts.IUniswapV3Pool, poolAddress string, tickSpacing int32) error {
	if tickSpacing <= 0 {
		return fmt.Errorf("invalid tickSpacing: %d", tickSpacing)
	}

	// 计算压缩 tick 和 word 的边界范围
	// Go 整除对正数取 floor、对负数取 ceiling，恰好与 V3 tickBitmap 的合法 tick 范围吻合
	minCompressed := int32(minTick) / tickSpacing
	maxCompressed := int32(maxTick) / tickSpacing
	minWord := int16(minCompressed >> 8) // 算术右移 = floor(/ 256)
	maxWord := int16(maxCompressed >> 8)

	callOpts := &bind.CallOpts{Context: ctx}
	one := big.NewInt(1)

	logger.Log.Info("Scanning tickBitmap",
		zap.String("pool", poolAddress),
		zap.Int16("minWord", minWord),
		zap.Int16("maxWord", maxWord))

	synced := 0
	for wordPos := minWord; wordPos <= maxWord; wordPos++ {
		if err := s.limiter.Wait(ctx); err != nil {
			return err
		}
		bitmap, err := contract.TickBitmap(callOpts, wordPos)
		if err != nil {
			return fmt.Errorf("tickBitmap word=%d: %w", wordPos, err)
		}
		if bitmap.Sign() == 0 {
			continue
		}

		// 遍历该 word 的 256 个 bit，找出已初始化的 tick
		for bit := 0; bit < 256; bit++ {
			mask := new(big.Int).Lsh(one, uint(bit))
			if new(big.Int).And(bitmap, mask).Sign() == 0 {
				continue
			}

			// compressed = wordPos*256 + bit，actualTick = compressed * tickSpacing
			compressed := int32(wordPos)*256 + int32(bit)
			tick := compressed * tickSpacing

			if err := s.limiter.Wait(ctx); err != nil {
				return err
			}
			tickData, err := contract.Ticks(callOpts, big.NewInt(int64(tick)))
			if err != nil {
				logger.Log.Error("Failed to get tick data",
					zap.String("pool", poolAddress),
					zap.Int32("tick", tick),
					zap.Error(err))
				continue
			}

			if !tickData.Initialized {
				continue
			}

			if err := s.upsertTick(ctx, &sqlmodel.AlphaAmmV3PoolTick{
				ChainID:               s.chainID,
				PoolAddress:           poolAddress,
				TickIndex:             tick,
				LiquidityGross:        tickData.LiquidityGross.String(),
				LiquidityNet:          tickData.LiquidityNet.String(),
				FeeGrowthOutside0X128: tickData.FeeGrowthOutside0X128.String(),
				FeeGrowthOutside1X128: tickData.FeeGrowthOutside1X128.String(),
				LastUpdatedAt:         time.Now(),
			}); err != nil {
				logger.Log.Error("Failed to upsert tick",
					zap.String("pool", poolAddress),
					zap.Int32("tick", tick),
					zap.Error(err))
			} else {
				synced++
			}
		}
	}

	logger.Log.Info("Tick sync completed",
		zap.String("pool", poolAddress),
		zap.Int("synced", synced))
	return nil
}

// upsertPool 按 pool_address 查询现有记录，存在则更新，不存在则新增
func (s *PoolSyncer) upsertPool(ctx context.Context, pool *sqlmodel.AlphaAmmPool) error {
	existing, err := dao.AlphaAmmPool.WithContext(ctx).
		Where(dao.AlphaAmmPool.PoolAddress.Eq(pool.PoolAddress)).
		First()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if existing == nil {
		return dao.AlphaAmmPool.WithContext(ctx).Create(pool)
	}

	pool.ID = existing.ID
	pool.CreatedAt = existing.CreatedAt
	return dao.AlphaAmmPool.WithContext(ctx).Save(pool)
}

// upsertTick 按 (pool_address, tick_index) 做 upsert，依赖表上的唯一索引
func (s *PoolSyncer) upsertTick(ctx context.Context, tick *sqlmodel.AlphaAmmV3PoolTick) error {
	return database.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "pool_address"},
				{Name: "tick_index"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"liquidity_gross",
				"liquidity_net",
				"fee_growth_outside0_x128",
				"fee_growth_outside1_x128",
				"last_updated_at",
			}),
		}).
		Create(tick).Error
}
