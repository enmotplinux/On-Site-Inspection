// Package model 定义巡检工具的核心数据模型。
package model

import "time"

// Instance 巡检目标 MySQL 实例。
type Instance struct {
	ID        string    `bson:"_id"` // 唯一标识：host_port
	Host      string    `bson:"host"`
	Port      int       `bson:"port"`
	User      string    `bson:"user"`
	Password  string    `bson:"password"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// InspectionResult 单个实例一次巡检的完整结果。
type InspectionResult struct {
	InstanceID    string                `bson:"instance_id"`
	Host          string                `bson:"host"`
	Port          int                   `bson:"port"`
	CollectedAt   time.Time             `bson:"collected_at"`
	DurationMS    int64                 `bson:"duration_ms"`
	SampleSeconds int                   `bson:"sample_seconds"` // 状态差值对应的采样间隔
	Error         string                `bson:"error,omitempty"`
	Variables     map[string]string     `bson:"variables,omitempty"`     // SHOW GLOBAL VARIABLES 中关注的键
	StatusDelta   map[string]int64      `bson:"status_delta,omitempty"`  // 采样间隔内的增量（QPS/TPS 等）
	StatusRaw     map[string]string     `bson:"status_raw,omitempty"`   // 累计值（锁等待、连接数等）
	Replication   *ReplicationStatus    `bson:"replication,omitempty"`  // 复制状态（从库才有）
	AutoIncrement []TableAutoIncrement  `bson:"auto_increment,omitempty"` // 业务表自增值（降序）
}

// ReplicationStatus SHOW SLAVE STATUS / SHOW REPLICA STATUS 的关键字段。
type ReplicationStatus struct {
	Role                 string           `bson:"role"` // slave / replica / none
	MasterHost           string           `bson:"master_host,omitempty"`
	MasterPort           string           `bson:"master_port,omitempty"`
	IORunning            string           `bson:"io_running,omitempty"`
	SQLRunning           string           `bson:"sql_running,omitempty"`
	SecondsBehindMaster  string           `bson:"seconds_behind_master,omitempty"`
	LastIOError          string           `bson:"last_io_error,omitempty"`
	LastSQLError         string           `bson:"last_sql_error,omitempty"`
	Raw                  map[string]string `bson:"raw,omitempty"` // 原始行，列名 -> 值
}

// TableAutoIncrement 业务表自增 ID 使用情况。
type TableAutoIncrement struct {
	Schema        string `bson:"schema"`
	Table         string `bson:"table"`
	Engine        string `bson:"engine"`
	AutoIncrement *int64 `bson:"auto_increment"` // NULL 时为 nil
}

// Summary 单实例采集结果摘要（用于命令行输出）。
type Summary struct {
	InstanceID string
	Duration   time.Duration
	Err        error
}
