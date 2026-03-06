
## 功能模块

### uniswap V2 vs V3 滑点差异分析
模拟不同交易规模下两种 AMM 的滑点差异，并提供可视化曲线。面向交易者和 LP，有实际决策价值。
- 历史交易数据（Swap events）
- 当前池子流动性信息（reserve、tick info）
- Token decimal 和价格信息
- 公式：Uniswap V2: Δy = 997y * Δx / (1000x + 997Δx); Uniswap V3: 根据 ticks 计算 sqrtPriceImpact

### LP APY 预测模块
根据手续费分布和历史交易量预测未来 LP 收益，支持 V2 和 V3 不同 fee tier。
- 历史手续费增长（feeGrowthGlobal / feeGrowthOutside）
- 每个 LP 的持仓 tick 上下限、流动性（liquidity）
- Token价格
- 计算公式：收益 = 用户流动性占比 × 手续费增长

### 实时可落地交易策略生成
基于链上实时数据和分析结果，提示套利机会、拆单策略或最优 swap 路径。可以直接对接交易执行模块。
- 当前交易池状态（liquidity, tick, sqrtPriceX96）
- Token余额和价格
- 历史滑点分析结果
- 算法：寻找跨池套利、V2/V3 最优交易路径、拆单以降低滑点
