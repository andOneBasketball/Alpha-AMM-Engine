package service

import (
	"context"
	"math/big"

	"alpha-amm-engine/internal/contracts"
	"alpha-amm-engine/pkg/logger"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var q96 = new(big.Int).Lsh(big.NewInt(1), 96)
var q96D = decimal.NewFromBigInt(new(big.Int).Lsh(big.NewInt(1), 96), 0)

const maxTickVal = int32(887272)

type v3TickEntry struct {
	tick         int32
	sqrtPriceX96 *big.Int
	liquidityNet *big.Int // int128，可为负
}

type v3SwapSim struct {
	sqrtPriceX96 *big.Int
	liquidity    *big.Int
	currentTick  int32
	tickSpacing  int32
	fee          int64 // 手续费 ppm，例如 3000 = 0.3%

	caller      *contracts.IUniswapV3PoolCaller
	ctx         context.Context
	bitmapCache map[int16]*big.Int    // wordPos -> bitmap，避免重复 RPC
	tickCache   map[int32]v3TickEntry // tick -> entry，避免重复 RPC
}

// tickToSqrtPriceX96 计算 sqrt(1.0001^tick) * 2^96
var base1_0001, _ = decimal.NewFromString("1.0001")

func tickToSqrtPriceX96(tick int32) *big.Int {
	exp := decimal.NewFromInt(int64(tick)).Div(decimal.NewFromInt(2))
	sqrtP := base1_0001.Pow(exp)
	return sqrtP.Mul(q96D).BigInt()
}

// amount0Delta 计算价格从 sqrtA 移动到 sqrtB 时的 |Δtoken0|
// 公式：liquidity * |sqrtB - sqrtA| * Q96 / (sqrtA * sqrtB)
func amount0Delta(sqrtA, sqrtB, liquidity *big.Int) *big.Int {
	if sqrtA.Cmp(sqrtB) > 0 {
		sqrtA, sqrtB = sqrtB, sqrtA
	}
	diff := new(big.Int).Sub(sqrtB, sqrtA)
	num := new(big.Int).Mul(new(big.Int).Mul(liquidity, diff), q96)
	denom := new(big.Int).Mul(sqrtA, sqrtB)
	if denom.Sign() == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(num, denom)
}

// amount1Delta 计算价格从 sqrtA 移动到 sqrtB 时的 |Δtoken1|
// 公式：liquidity * |sqrtB - sqrtA| / Q96
func amount1Delta(sqrtA, sqrtB, liquidity *big.Int) *big.Int {
	if sqrtA.Cmp(sqrtB) > 0 {
		sqrtA, sqrtB = sqrtB, sqrtA
	}
	diff := new(big.Int).Sub(sqrtB, sqrtA)
	return new(big.Int).Div(new(big.Int).Mul(liquidity, diff), q96)
}

// newSqrtPriceAfterAmount0In 计算注入 amount0 token0 后的新 sqrtPrice（价格下跌）
// 公式：newSqrt = liquidity * Q96 * sqrtPrice / (liquidity * Q96 + amount0 * sqrtPrice)
func newSqrtPriceAfterAmount0In(sqrtPrice, liquidity, amount0 *big.Int) *big.Int {
	liq96 := new(big.Int).Mul(liquidity, q96)
	num := new(big.Int).Mul(liq96, sqrtPrice)
	denom := new(big.Int).Add(liq96, new(big.Int).Mul(amount0, sqrtPrice))
	if denom.Sign() == 0 {
		return new(big.Int).Set(sqrtPrice)
	}
	return new(big.Int).Div(num, denom)
}

// newSqrtPriceAfterAmount1In 计算注入 amount1 token1 后的新 sqrtPrice（价格上涨）
// 公式：newSqrt = sqrtPrice + amount1 * Q96 / liquidity
func newSqrtPriceAfterAmount1In(sqrtPrice, liquidity, amount1 *big.Int) *big.Int {
	delta := new(big.Int).Div(new(big.Int).Mul(amount1, q96), liquidity)
	return new(big.Int).Add(sqrtPrice, delta)
}

type v3PoolState struct {
	sqrtPriceX96 *big.Int
	currentTick  int32
	liquidity    *big.Int
	tickSpacing  int32
	fee          int64
}

// loadV3PoolState 一次性加载池子的静态状态（slot0 / liquidity / tickSpacing / fee）
func loadV3PoolState(ctx context.Context, caller *contracts.IUniswapV3PoolCaller) (*v3PoolState, error) {
	opts := &bind.CallOpts{Context: ctx}

	slot0, err := caller.Slot0(opts)
	if err != nil {
		return nil, err
	}
	liq, err := caller.Liquidity(opts)
	if err != nil {
		return nil, err
	}
	tickSpacingRaw, err := caller.TickSpacing(opts)
	if err != nil {
		return nil, err
	}
	feeRaw, err := caller.Fee(opts)
	if err != nil {
		return nil, err
	}
	v3State := &v3PoolState{
		sqrtPriceX96: new(big.Int).Set(slot0.SqrtPriceX96),
		currentTick:  int32(slot0.Tick.Int64()),
		liquidity:    new(big.Int).Set(liq),
		tickSpacing:  int32(tickSpacingRaw.Int64()),
		fee:          feeRaw.Int64(),
	}

	sqrtPriceD := decimal.NewFromBigInt(v3State.sqrtPriceX96, 0)
	q96D := decimal.NewFromBigInt(q96, 0)
	v3Price := sqrtPriceD.Div(q96D).Pow(decimal.NewFromInt(2))

	logger.Log.Info("Loaded V3 pool state",
		zap.String("v3Price1", v3Price.String()),
		zap.String("v3Price2", decimal.NewFromInt32(1).Div(v3Price).StringFixed(18)),
		zap.Int32("currentTick", int32(slot0.Tick.Int64())),
		zap.String("liquidity", liq.String()),
		zap.Int32("tickSpacing", int32(tickSpacingRaw.Int64())),
		zap.Int64("fee", feeRaw.Int64()),
	)
	return v3State, nil
}

// loadV3SwapSim 创建 V3 swap 模拟器，不做任何预加载，simulate 时按需查询
func loadV3SwapSim(ctx context.Context, caller *contracts.IUniswapV3PoolCaller, state *v3PoolState) (*v3SwapSim, error) {
	return &v3SwapSim{
		sqrtPriceX96: new(big.Int).Set(state.sqrtPriceX96),
		liquidity:    new(big.Int).Set(state.liquidity),
		currentTick:  state.currentTick,
		tickSpacing:  state.tickSpacing,
		fee:          state.fee,
		caller:       caller,
		ctx:          ctx,
		bitmapCache:  make(map[int16]*big.Int),
		tickCache:    make(map[int32]v3TickEntry),
	}, nil
}

// fetchTickBitmap 从缓存或链上获取 tickBitmap[wordPos]
func (sim *v3SwapSim) fetchTickBitmap(wordPos int16) (*big.Int, error) {
	if v, ok := sim.bitmapCache[wordPos]; ok {
		return v, nil
	}
	bitmap, err := sim.caller.TickBitmap(&bind.CallOpts{Context: sim.ctx}, wordPos)
	if err != nil {
		return nil, err
	}
	sim.bitmapCache[wordPos] = new(big.Int).Set(bitmap)
	logger.Log.Info("fetch tick bitmap", zap.Int16("wordPos", wordPos), zap.String("bitmap", bitmap.Text(16)))
	return sim.bitmapCache[wordPos], nil
}

// loadTickEntry 从缓存或链上加载单个 tick 数据
func (sim *v3SwapSim) loadTickEntry(tick int32) (v3TickEntry, error) {
	if entry, ok := sim.tickCache[tick]; ok {
		return entry, nil
	}
	td, err := sim.caller.Ticks(&bind.CallOpts{Context: sim.ctx}, big.NewInt(int64(tick)))
	if err != nil {
		return v3TickEntry{}, err
	}
	entry := v3TickEntry{
		tick:         tick,
		sqrtPriceX96: tickToSqrtPriceX96(tick),
		liquidityNet: new(big.Int).Set(td.LiquidityNet),
	}
	sim.tickCache[tick] = entry
	return entry, nil
}

// nextInitializedTick 通过 tickBitmap（带缓存）在指定方向查找最近的已初始化 tick
// zeroForOne=true：找 <= currentTick 的最近已初始化 tick（价格下跌方向）
// zeroForOne=false：找 > currentTick 的最近已初始化 tick（价格上涨方向）
func (sim *v3SwapSim) nextInitializedTick(currentTick int32, zeroForOne bool) (int32, bool, error) {
	compressed := currentTick / sim.tickSpacing
	// 对负 tick 做 floor division 修正，与 Uniswap V3 合约行为一致
	if currentTick < 0 && currentTick%sim.tickSpacing != 0 {
		compressed--
	}

	// tick 范围对应的 wordPos 边界，防止无限循环
	minWordPos := int16(-maxTickVal/int32(sim.tickSpacing)/256 - 2)
	maxWordPos := int16(maxTickVal/int32(sim.tickSpacing)/256 + 2)

	if zeroForOne {
		wordPos := int16(compressed >> 8)
		bitPos := uint(uint32(compressed) & 0xFF)

		for wordPos >= minWordPos {
			bitmap, err := sim.fetchTickBitmap(wordPos)
			if err != nil {
				return 0, false, err
			}
			if bitmap.Sign() != 0 {
				// 找 bitmap 中 <= bitPos 的最高设置位
				// mask = (2^(bitPos+1)) - 1，覆盖 0..bitPos
				var mask *big.Int
				if bitPos >= 255 {
					mask = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
				} else {
					mask = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), bitPos+1), big.NewInt(1))
				}
				masked := new(big.Int).And(bitmap, mask)
				if masked.Sign() != 0 {
					bit := uint(masked.BitLen() - 1)
					foundTick := (int32(wordPos)*256 + int32(bit)) * sim.tickSpacing
					return foundTick, true, nil
				}
			}
			// 当前 word 没有，向下查下一个 word（全 256 位）
			wordPos--
			bitPos = 255
		}
		return 0, false, nil
	}

	// zeroForOne=false：找 > currentTick 的 tick，从 compressed+1 开始
	compressed++
	wordPos := int16(compressed >> 8)
	bitPos := uint(uint32(compressed) & 0xFF)

	for wordPos <= maxWordPos {
		bitmap, err := sim.fetchTickBitmap(wordPos)
		if err != nil {
			return 0, false, err
		}
		if bitmap.Sign() != 0 {
			// 找 bitmap 中 >= bitPos 的最低设置位
			// 将 bitmap 右移 bitPos 位后取 TrailingZeroBits
			shifted := new(big.Int).Rsh(bitmap, bitPos)
			if shifted.Sign() != 0 {
				bit := uint(shifted.TrailingZeroBits()) + bitPos
				return (int32(wordPos)*256 + int32(bit)) * sim.tickSpacing, true, nil
			}
		}
		// 当前 word 没有，向上查下一个 word（从第 0 位开始）
		wordPos++
		bitPos = 0
	}
	return 0, false, nil
}

// simulate 从当前池子状态模拟兑换 amountInRaw（链上原始整数单位）并返回 amountOutRaw。
// 直接在当前流动性区间计算；只有当 amountIn 耗尽当前区间时，才通过 tickBitmap 按需
// 查找下一个已初始化 tick，加载其数据后继续计算。
// bitmap 和 tick 数据均有缓存，50 次 simulate 调用可复用，不重复 RPC。
func (sim *v3SwapSim) simulate(zeroForOne bool, amountInRaw *big.Int) *big.Int {
	// 扣除手续费：amountInNet = amountIn * (1e6 - fee) / 1e6
	amountInNet := new(big.Int).Div(
		new(big.Int).Mul(amountInRaw, big.NewInt(1_000_000-sim.fee)),
		big.NewInt(1_000_000),
	)

	amountRemaining := new(big.Int).Set(amountInNet)
	totalOut := new(big.Int)
	sqrtPrice := new(big.Int).Set(sim.sqrtPriceX96)
	liquidity := new(big.Int).Set(sim.liquidity)
	currentTick := sim.currentTick

	for amountRemaining.Sign() > 0 {
		if liquidity.Sign() == 0 {
			break
		}

		// 通过 tickBitmap（缓存）找下一个已初始化 tick，大多数循环无 RPC 开销
		nextTick, found, err := sim.nextInitializedTick(currentTick, zeroForOne)
		if err != nil || !found {
			// 无更多 tick：当前流动性区间无边界，直接消耗全部剩余 amountIn
			if zeroForOne {
				newSqrt := newSqrtPriceAfterAmount0In(sqrtPrice, liquidity, amountRemaining)
				totalOut.Add(totalOut, amount1Delta(newSqrt, sqrtPrice, liquidity))
			} else {
				newSqrt := newSqrtPriceAfterAmount1In(sqrtPrice, liquidity, amountRemaining)
				totalOut.Add(totalOut, amount0Delta(sqrtPrice, newSqrt, liquidity))
			}
			break
		}

		sqrtTarget := tickToSqrtPriceX96(nextTick)

		// 计算到达 tick 边界所需的最大 amountIn
		var maxIn *big.Int
		if zeroForOne {
			maxIn = amount0Delta(sqrtTarget, sqrtPrice, liquidity)
		} else {
			maxIn = amount1Delta(sqrtPrice, sqrtTarget, liquidity)
		}

		if amountRemaining.Cmp(maxIn) <= 0 {
			// 不穿越 tick：在当前流动性区间内部分消耗
			if zeroForOne {
				newSqrt := newSqrtPriceAfterAmount0In(sqrtPrice, liquidity, amountRemaining)
				totalOut.Add(totalOut, amount1Delta(newSqrt, sqrtPrice, liquidity))
			} else {
				newSqrt := newSqrtPriceAfterAmount1In(sqrtPrice, liquidity, amountRemaining)
				totalOut.Add(totalOut, amount0Delta(sqrtPrice, newSqrt, liquidity))
			}
			break
		}

		// 穿越 tick：消耗整个区间直到 tick 边界
		if zeroForOne {
			totalOut.Add(totalOut, amount1Delta(sqrtTarget, sqrtPrice, liquidity))
		} else {
			totalOut.Add(totalOut, amount0Delta(sqrtPrice, sqrtTarget, liquidity))
		}
		amountRemaining.Sub(amountRemaining, maxIn)
		sqrtPrice.Set(sqrtTarget)

		// 按需加载 tick 数据（首次加载后缓存，后续 simulate 直接复用）
		entry, err := sim.loadTickEntry(nextTick)
		if err != nil {
			logger.Log.Warn("load tick entry failed", zap.Int32("tick", nextTick), zap.Error(err))
			break
		}

		// 向下穿越：subtract liquidityNet；向上穿越：add liquidityNet
		if zeroForOne {
			liquidity.Sub(liquidity, entry.liquidityNet)
		} else {
			liquidity.Add(liquidity, entry.liquidityNet)
		}
		if liquidity.Sign() < 0 {
			liquidity.SetInt64(0)
		}

		price0 := decimal.NewFromBigInt(sqrtPrice, 0).Div(q96D).Pow(decimal.NewFromInt(2))
		price1 := decimal.NewFromInt32(1).Div(price0)
		logger.Log.Info("cross tick", zap.Int32("tick", nextTick), zap.String("sqrtPriceX96", sqrtPrice.String()), zap.String("price0", price0.String()), zap.String("price1", price1.String()), zap.String("liquidity", liquidity.String()), zap.String("amountRemaining", amountRemaining.String()), zap.String("totalOut", totalOut.String()), zap.Bool("zeroForOne", zeroForOne))
		// 更新 currentTick，供下次 nextInitializedTick 查询定位
		if zeroForOne {
			currentTick = nextTick - 1
		} else {
			currentTick = nextTick
		}
	}

	return totalOut
}
