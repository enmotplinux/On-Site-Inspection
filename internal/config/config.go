// Package config 负责巡检工具配置的加载、默认值填充与校验。
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config 巡检工具总配置。
type Config struct {
	MongoDB MongoDBConfig `json:"mongodb"`
	Collect CollectConfig `json:"collect"`
}

// MongoDBConfig MongoDB 连接配置。
type MongoDBConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	Database     string `json:"database"`      // 巡检数据所在库
	AuthDatabase string `json:"auth_database"` // 认证库（对应旧版 conn.<db>.authenticate 的库名）
}

// CollectConfig 采集行为配置。
type CollectConfig struct {
	Concurrency            int           `json:"concurrency"`              // 并发采集的实例数
	SampleIntervalSeconds  int           `json:"sample_interval_seconds"`  // SHOW GLOBAL STATUS 两次采样的间隔
	ConnectTimeoutSeconds  int           `json:"connect_timeout_seconds"`  // MySQL 建连超时
	ReadTimeoutSeconds     int           `json:"read_timeout_seconds"`     // MySQL 查询读超时
	ResultRetention        int           `json:"result_retention"`         // 每个实例保留的历史结果份数
	AutoIncrementTopN      int           `json:"auto_increment_top_n"`    // 报告展示自增值最大的前 N 张表
}

// SampleInterval 采样间隔（Duration 形式）。
func (c CollectConfig) SampleInterval() time.Duration {
	return time.Duration(c.SampleIntervalSeconds) * time.Second
}

// ConnectTimeout MySQL 建连超时。
func (c CollectConfig) ConnectTimeout() time.Duration {
	return time.Duration(c.ConnectTimeoutSeconds) * time.Second
}

// ReadTimeout MySQL 查询读超时。
func (c CollectConfig) ReadTimeout() time.Duration {
	return time.Duration(c.ReadTimeoutSeconds) * time.Second
}

// Default 返回一份带默认值的配置。
func Default() Config {
	return Config{
		MongoDB: MongoDBConfig{
			Host:         "127.0.0.1",
			Port:         27017,
			Database:     "onsite_inspection",
			AuthDatabase: "admin",
		},
		Collect: CollectConfig{
			Concurrency:           5,
			SampleIntervalSeconds: 1,
			ConnectTimeoutSeconds: 5,
			ReadTimeoutSeconds:    10,
			ResultRetention:       20,
			AutoIncrementTopN:     10,
		},
	}
}

// URI 生成 MongoDB 连接 URI。
func (m MongoDBConfig) URI() string {
	uri := &url.URL{
		Scheme: "mongodb",
		Host:   m.Host + ":" + strconv.Itoa(m.Port),
	}
	if m.User != "" {
		if m.Password != "" {
			uri.User = url.UserPassword(m.User, m.Password)
		} else {
			uri.User = url.User(m.User)
		}
		uri.Path = m.AuthDatabase
	}
	return uri.String()
}

// Load 从 path 读取配置：先以 Default 填充默认值，再覆盖 JSON 字段，
// 最后用环境变量覆盖敏感字段（避免配置文件里出现明文密码）。
// ONSITE_MONGO_PASSWORD  覆盖 MongoDB 密码
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path) // #nosec G304 -- 路径来自用户指定的 -config 参数
	if err != nil {
		return cfg, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("解析配置文件 %s: %w", path, err)
	}

	if v := os.Getenv("ONSITE_MONGO_PASSWORD"); v != "" {
		cfg.MongoDB.Password = v
	}

	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("配置校验失败: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.MongoDB.Host == "" {
		return fmt.Errorf("mongodb.host 不能为空")
	}
	if c.MongoDB.Port <= 0 {
		return fmt.Errorf("mongodb.port 必须为正整数")
	}
	if c.MongoDB.Database == "" {
		return fmt.Errorf("mongodb.database 不能为空")
	}
	if c.Collect.Concurrency <= 0 {
		return fmt.Errorf("collect.concurrency 必须为正整数")
	}
	if c.Collect.SampleIntervalSeconds <= 0 {
		return fmt.Errorf("collect.sample_interval_seconds 必须为正整数")
	}
	if c.Collect.ResultRetention <= 0 {
		return fmt.Errorf("collect.result_retention 必须为正整数")
	}
	return nil
}

// ResolvePath 解析配置文件路径，查找顺序：
// 1. 环境变量 ONSITE_CONFIG
// 2. 当前目录 conf/config.json
// 3. 可执行文件同级 conf/config.json（部署形态：bin + conf）
// 4. 可执行文件上级 conf/config.json（开发形态：cmd/onsite + conf）
func ResolvePath(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("指定的配置文件不存在: %w", err)
		}
		return explicit, nil
	}

	candidates := []string{}
	if v := os.Getenv("ONSITE_CONFIG"); v != "" {
		candidates = append(candidates, v)
	}
	candidates = append(candidates, filepath.Join("conf", "config.json"))
	if exec, err := os.Executable(); err == nil {
		execDir := filepath.Dir(exec)
		candidates = append(candidates,
			filepath.Join(execDir, "conf", "config.json"),
			filepath.Join(execDir, "..", "conf", "config.json"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到配置文件，可通过 -config 参数或 ONSITE_CONFIG 环境变量指定（已尝试: %v）", candidates)
}
