package service

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"

	"alpha-amm-engine/internal/contracts"
	"alpha-amm-engine/internal/dao"
	"alpha-amm-engine/pkg/config"
	"alpha-amm-engine/pkg/logger"
	"alpha-amm-engine/pkg/models"
	"alpha-amm-engine/svc/handler"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type tokenMeta struct {
	Symbol   string
	Decimals int32
}

// SlippageCurve 根据链上真实储备量，生成 Uniswap V2 双向平均成交价格曲线
func SlippageCurve(ctx context.Context, req *models.SlippageCurveReq) (*models.CommonResp, error) {
	chainCfg, ok := config.Cfg.Scan.Blockchain[req.ChainId]
	if !ok {
		return nil, fmt.Errorf("blockchain config for chain %s not found", req.ChainId)
	}

	pairAddr, isSwapped := handler.UniswapV2PairFor(req.Token0Addr, req.Token1Addr)

	// 查询两个代币的 symbol 和 decimals（按用户传入的地址顺序查询，与 isSwapped 无关）
	meta0 := queryTokenMeta(ctx, req.Token0Addr)
	meta1 := queryTokenMeta(ctx, req.Token1Addr)

	ethClient, err := ethclient.Dial(chainCfg.RPC)
	if err != nil {
		return nil, fmt.Errorf("dial rpc failed: %w", err)
	}
	defer ethClient.Close()

	pairCaller, err := contracts.NewIUniswapV2PairCaller(pairAddr, ethClient)
	if err != nil {
		return nil, fmt.Errorf("create pair caller failed: %w", err)
	}

	reserves, err := pairCaller.GetReserves(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("getReserves failed (pair %s): %w", pairAddr.Hex(), err)
	}

	// pair 合约按地址数值升序存储；isSwapped=true 时 Reserve0 对应用户的 token1
	r0, r1 := reserves.Reserve0, reserves.Reserve1
	if isSwapped {
		r0, r1 = r1, r0
	}

	if r0.Sign() == 0 || r1.Sign() == 0 {
		return nil, fmt.Errorf("pair %s has no liquidity", pairAddr.Hex())
	}

	stepSize0 := req.SamplingStep0
	if !stepSize0.IsPositive() {
		stepSize0 = decimal.NewFromInt(1)
	}
	stepSize1 := req.SamplingStep1
	if !stepSize1.IsPositive() {
		stepSize1 = decimal.NewFromInt(1)
	}

	// 以 decimals 调整后的人类可读储备量
	r0D := decimal.NewFromBigInt(r0, -meta0.Decimals)
	r1D := decimal.NewFromBigInt(r1, -meta1.Decimals)

	chart0 := buildPriceChart(r0, r1, meta0.Decimals, meta1.Decimals, stepSize0, meta0.Symbol, meta1.Symbol, "uniswap v2")
	chart1 := buildPriceChart(r1, r0, meta1.Decimals, meta0.Decimals, stepSize1, meta1.Symbol, meta0.Symbol, "uniswap v2")

	// 如果提供了 V3 fee，通过 CREATE2 计算 pool 地址并叠加 V3 曲线到同一张图
	var (
		v3State     *v3PoolState
		v3PoolAddr  common.Address
		v3IsSwapped bool
	)
	if req.V3Fee > 0 {
		v3PoolAddr, v3IsSwapped = handler.UniswapV3PoolFor(req.Token0Addr, req.Token1Addr, req.V3Fee)
		v3Caller, err := contracts.NewIUniswapV3PoolCaller(v3PoolAddr, ethClient)
		if err == nil {
			// 一次性加载 slot0 / liquidity / tickSpacing / fee，两个方向共用
			if v3State, err = loadV3PoolState(ctx, v3Caller); err == nil {
				// v3IsSwapped=true 表示 pool.token0 = user.token1，即 chart0 方向为 oneForZero
				v3ForwardIsZeroForOne := !v3IsSwapped
				v3Sim, err := loadV3SwapSim(ctx, v3Caller, v3State)
				if err != nil {
					logger.Log.Errorf("loadV3SwapSim failed for pool %s: %v", v3PoolAddr.Hex(), err)
				} else {
					chart0.AddSeries("uniswap v3", buildV3YData(v3Sim, v3ForwardIsZeroForOne, stepSize0, meta0.Decimals, meta1.Decimals))
					chart1.AddSeries("uniswap v3", buildV3YData(v3Sim, !v3ForwardIsZeroForOne, stepSize1, meta1.Decimals, meta0.Decimals))
				}
			}
		}
	}

	page := components.NewPage()
	page.AddCharts(chart0, chart1)

	var buf bytes.Buffer
	if err = page.Render(&buf); err != nil {
		return nil, err
	}

	// page.Render 生成完整 HTML 文档，将自定义 header 注入到 <body> 内

	// V2 流动性 L = sqrt(r0 * r1)（人类可读单位）
	v2Liquidity := handler.DecimalSqrt(r0D.Mul(r1D))

	headerHTML := fmt.Sprintf(`<h3>Alpha AMM Engine</h3>
<p>
AMM liquidity analytics tool.
<br>
Analyzes swap price curves and slippage under different trade sizes.
<br>
uniswap v2 pool: %s reserve: %s, %s reserve: %s, liquidity: %s`,
		meta0.Symbol, r0D.String(), meta1.Symbol, r1D.String(), v2Liquidity.String())

	if v3State != nil {
		// price(token1/token0) = (sqrtPriceX96 / 2^96)^2
		sqrtPriceD := decimal.NewFromBigInt(v3State.sqrtPriceX96, 0)
		q96D := decimal.NewFromBigInt(q96, 0)
		v3Price0 := sqrtPriceD.Div(q96D).Pow(decimal.NewFromInt(2))
		v3Price1 := decimal.NewFromInt(1).Div(v3Price0)
		headerHTML += fmt.Sprintf(`
<br>
uniswap v3 pool: price0 %s, price1 %s %s/%s, liquidity: %s`,
			v3Price0.StringFixed(8), v3Price1.StringFixed(8), meta1.Symbol, meta0.Symbol, v3State.liquidity.String())
	}

	headerHTML += `
</p>
<hr>
`
	htmlContent := strings.Replace(buf.String(), "<body>", "<body>"+headerHTML, 1)

	f, _ := os.Create("amm_slippage_curve.html")
	f.WriteString(htmlContent)

	logger.Log.Info("calculate slippage curve success", zap.String("pair_v2", pairAddr.Hex()), zap.String("pair_v3", v3PoolAddr.Hex()), zap.String("token0", meta0.Symbol), zap.String("token1", meta1.Symbol))

	return &models.CommonResp{
		Data: htmlContent,
	}, nil
}

// buildPriceChart 构建单向平均成交价格折线图
//
// 输入单位均为链上原始整数（raw），内部通过 decimals 转换为人类可读量再做计算。
// Uniswap V2 公式在人类可读量下形式不变：
//
//	amountOut = rOut * amountIn * 997 / (rIn * 1000 + amountIn * 997)
//
// stepSize 为每步的 token 数量（人类可读单位），固定采样 50 步。
func buildPriceChart(rIn, rOut *big.Int, decIn, decOut int32, stepSize decimal.Decimal, symIn, symOut, lineName string) *charts.Line {
	// 除以 10^decimals，转为人类可读储备量
	rInD := decimal.NewFromBigInt(rIn, -decIn)
	rOutD := decimal.NewFromBigInt(rOut, -decOut)

	spotPrice := rOutD.Div(rInD)

	const totalSteps = 50
	d997 := decimal.NewFromInt(997)
	d1000 := decimal.NewFromInt(1000)

	xData := make([]string, totalSteps)
	yData := make([]opts.LineData, totalSteps)

	for i := int64(1); i <= totalSteps; i++ {
		amountIn := stepSize.Mul(decimal.NewFromInt(i))

		var avgPrice decimal.Decimal
		if amountIn.IsZero() {
			avgPrice = spotPrice
		} else {
			num := rOutD.Mul(amountIn).Mul(d997)
			denom := rInD.Mul(d1000).Add(amountIn.Mul(d997))
			avgPrice = num.Div(denom).Div(amountIn)
		}

		xData[i-1] = amountIn.String()
		yData[i-1] = opts.LineData{Value: avgPrice.InexactFloat64()}
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    fmt.Sprintf("%s → %s  Avg Execution Price", symIn, symOut),
			Subtitle: fmt.Sprintf("Spot: %s %s/%s", spotPrice.StringFixed(8), symOut, symIn),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name:         fmt.Sprintf("Input Amount (%s)", symIn),
			NameLocation: "middle",
			NameGap:      30,
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Avg Price (%s/%s)", symOut, symIn),
		}),
		charts.WithTooltipOpts(opts.Tooltip{Trigger: "axis"}),
		charts.WithGridOpts(opts.Grid{Top: "80px", Bottom: "80px"}),
	)
	line.SetXAxis(xData).AddSeries(lineName, yData)
	return line
}

// queryTokenMeta 从 alpha_token 表按地址查 symbol 和 decimals
// 查不到则降级：symbol 用地址缩写，decimals 默认 18
func queryTokenMeta(ctx context.Context, addr string) tokenMeta {
	token, err := dao.AlphaToken.WithContext(ctx).
		Where(dao.AlphaToken.Address.Eq(strings.ToLower(addr))).
		First()
	if err != nil {
		return tokenMeta{Symbol: shortAddr(addr), Decimals: 18}
	}
	return tokenMeta{Symbol: token.Symbol, Decimals: token.Decimals}
}

func shortAddr(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

// buildV3YData 为 V3 swap 模拟生成 50 个平均成交价数据点（与 V2 图表 X 轴对齐）
// 优化策略：先计算最大兑换量预热缓存，再从大到小回溯计算，复用已缓存的 tick 数据
func buildV3YData(sim *v3SwapSim, zeroForOne bool, stepSize decimal.Decimal, decIn, decOut int32) []opts.LineData {
	const totalSteps = 50
	yData := make([]opts.LineData, totalSteps)

	// 第一步：计算最大兑换量（第 50 步），预热 tick 缓存
	maxAmountInHuman := stepSize.Mul(decimal.NewFromInt(totalSteps))
	maxAmountInRaw := maxAmountInHuman.Mul(decimal.New(1, decIn)).BigInt()
	maxAmountOutRaw := sim.simulate(zeroForOne, maxAmountInRaw)
	maxAmountOutHuman := decimal.NewFromBigInt(maxAmountOutRaw, -decOut)

	var maxAvgPrice float64
	if maxAmountInHuman.IsPositive() {
		maxAvgPrice, _ = maxAmountOutHuman.Div(maxAmountInHuman).Float64()
	}
	yData[totalSteps-1] = opts.LineData{Value: maxAvgPrice}

	// 第二步：从大到小回溯计算其他点，复用缓存
	for i := int64(totalSteps - 1); i >= 1; i-- {
		amountInHuman := stepSize.Mul(decimal.NewFromInt(i))
		amountInRaw := amountInHuman.Mul(decimal.New(1, decIn)).BigInt()

		amountOutRaw := sim.simulate(zeroForOne, amountInRaw)
		amountOutHuman := decimal.NewFromBigInt(amountOutRaw, -decOut)

		var avgPrice float64
		if amountInHuman.IsPositive() {
			avgPrice, _ = amountOutHuman.Div(amountInHuman).Float64()
		}
		yData[i-1] = opts.LineData{Value: avgPrice}
	}
	return yData
}
