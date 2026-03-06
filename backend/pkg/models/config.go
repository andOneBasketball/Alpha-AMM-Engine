package models

// Config 对应 conf.yaml 的整体配置
type Config struct {
	Debug bool        `yaml:"debug"`
	Env   string      `yaml:"env"`
	Web   Web         `yaml:"web"`
	Log   LogConfig   `yaml:"log"`
	MySQL MySQLConfig `yaml:"mysql"`
	Scan  ScanConfig  `yaml:"scan"`
}

type ScanConfig struct {
	Blockchain map[string]Blockchain `yaml:"blockchain"`
}

type Blockchain struct {
	RPC             string            `yaml:"rpc"`
	ChainID         int               `yaml:"chain_id"`
	StartBlock      int64             `yaml:"start_block"`
	SyncInterval    int               `yaml:"sync_interval"`
	CatchUpInterval int               `yaml:"catch_up_interval"`
	MaxQueryRange   int64             `yaml:"max_query_range"`
	MinQueryRange   int64             `yaml:"min_query_range"`
	Contracts       map[string]string `yaml:"contracts"`
}

type Web struct {
	Addr string `yaml:"addr"`
}

type LogConfig struct {
	Path string `yaml:"path"` // 日志文件路径
}

type MySQLConfig struct {
	Uri          string `yaml:"uri"`
	MaxPoolSize  int    `yaml:"max_pool_size"`
	IdlePoolSize int    `yaml:"idle_pool_size"`
	IdleTimeout  int    `yaml:"idle_timeout"`
	MaxLifetime  int    `yaml:"max_lifetime"`
}
