package scan

import (
	"context"
	"math/big"
	"sync"
	"time"

	"alpha-amm-engine/pkg/logger"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
)

type EthClient struct {
	url    string
	client *ethclient.Client
	mu     sync.RWMutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewEthClient(url string) (*EthClient, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}

	ec := &EthClient{
		url:    url,
		client: client,
		stopCh: make(chan struct{}),
	}

	// 启动健康检查
	ec.wg.Add(1)
	go ec.healthCheck()

	return ec, nil
}

func (c *EthClient) Close() {
	close(c.stopCh)
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client != nil {
		c.client.Close()
	}
}

// healthCheck 定期检查连接健康状态
func (c *EthClient) healthCheck() {
	defer c.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := c.getClient().HeaderByNumber(ctx, nil)
			cancel()

			if err != nil {
				logger.Log.Warn("ETH client health check failed, reconnecting...", zap.Error(err))
				c.reconnect()
			}
		}
	}
}

// reconnect 重新连接
func (c *EthClient) reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		c.client.Close()
	}

	for i := 0; i < 5; i++ {
		client, err := ethclient.Dial(c.url)
		if err == nil {
			c.client = client
			logger.Log.Info("ETH client reconnected successfully")
			return
		}

		logger.Log.Error("Failed to reconnect ETH client",
			zap.Int("attempt", i+1),
			zap.Error(err))
		time.Sleep(time.Second)
	}

	logger.Log.Error("Failed to reconnect ETH client after 5 attempts")
}

// getClient 获取客户端（带读锁）
func (c *EthClient) getClient() *ethclient.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

// GetBlockNumber 获取最新区块高度
func (c *EthClient) GetBlockNumber(ctx context.Context) (int64, error) {
	header, err := c.getClient().HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Int64(), nil
}

// GetLogs 获取日志
func (c *EthClient) GetLogs(ctx context.Context, fromBlock, toBlock int64, addresses []string) ([]types.Log, error) {
	funSelectors := []common.Hash{
		crypto.Keccak256Hash([]byte("Mint(address,uint256,uint256)")),
		crypto.Keccak256Hash([]byte("Burn(address,uint256,uint256)")),
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(fromBlock),
		ToBlock:   big.NewInt(toBlock),
		Topics:    [][]common.Hash{funSelectors},
	}

	if len(addresses) > 0 {
		addrs := make([]common.Address, len(addresses))
		for i, addr := range addresses {
			addrs[i] = common.HexToAddress(addr)
		}
		query.Addresses = addrs
	}

	return c.getClient().FilterLogs(ctx, query)
}

// GetBlockTime 获取区块时间
func (c *EthClient) GetBlockTime(ctx context.Context, blockNumber int64) (int64, error) {
	header, err := c.getClient().HeaderByNumber(ctx, big.NewInt(blockNumber))
	if err != nil {
		return 0, err
	}
	return int64(header.Time), nil
}
