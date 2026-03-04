package models

// Config 对应 conf.yaml 的整体配置
type Config struct {
	Debug bool        `yaml:"debug"`
	Env   string      `yaml:"env"`
	Web   Web         `yaml:"web"`
	Log   LogConfig   `yaml:"log"`
	MySQL MySQLConfig `yaml:"mysql"`
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
