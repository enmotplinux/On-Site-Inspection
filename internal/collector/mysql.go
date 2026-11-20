package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/enmotplinux/onsite-inspection/internal/config"
	"github.com/enmotplinux/onsite-inspection/internal/model"
)

// openMySQL 打开目标实例连接并验证连通性。
// 巡检只执行 SHOW / information_schema 查询，不需要指定默认库。
func openMySQL(ctx context.Context, inst model.Instance, cfg config.CollectConfig) (*sql.DB, error) {
	mc := mysql.NewConfig()
	mc.User = inst.User
	mc.Passwd = inst.Password
	mc.Net = "tcp"
	mc.Addr = net.JoinHostPort(inst.Host, strconv.Itoa(inst.Port))
	mc.Timeout = cfg.ConnectTimeout()
	mc.ReadTimeout = cfg.ReadTimeout()
	mc.Params = map[string]string{"charset": "utf8mb4"}

	db, err := sql.Open("mysql", mc.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("构建 DSN: %w", err)
	}
	// 短生命周期巡检连接：限制池大小，用完即关（CollectInstance 里 defer Close）
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接 %s: %w", inst.ID, err)
	}
	return db, nil
}

// collectVariables 采集关注的全局变量，缺失的变量自动跳过。
func collectVariables(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, fmt.Errorf("执行 SHOW GLOBAL VARIABLES: %w", err)
	}
	defer rows.Close()

	all := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("读取变量行: %w", err)
		}
		all[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历变量结果集: %w", err)
	}

	out := make(map[string]string, len(variableKeys))
	for _, k := range variableKeys {
		if v, ok := all[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

// collectStatus 两次采样 SHOW GLOBAL STATUS，返回差值与累计值。
func collectStatus(ctx context.Context, db *sql.DB, interval int) (map[string]int64, map[string]string, error) {
	before, err := queryGlobalStatus(ctx, db)
	if err != nil {
		return nil, nil, err
	}

	// 采样等待，期间响应取消信号
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-waitDuration(interval):
	}

	after, err := queryGlobalStatus(ctx, db)
	if err != nil {
		return nil, nil, err
	}

	return computeDelta(before, after), filterKeys(after, statusRawKeys), nil
}

func queryGlobalStatus(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW GLOBAL STATUS")
	if err != nil {
		return nil, fmt.Errorf("执行 SHOW GLOBAL STATUS: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("读取状态行: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// erParseError 是 MySQL 语法错误码（1064）。
// SHOW SLAVE STATUS 在 MySQL 8.4+ 被移除，执行时返回该错误码。
const erParseError = 1064

// isParseError 判断是否为 MySQL 语法错误（如 8.4+ 移除了 SHOW SLAVE STATUS）。
func isParseError(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == erParseError
}

// collectReplication 采集复制状态。
// 优先使用 SHOW SLAVE STATUS（5.x/8.0 兼容），语法错误时回退 SHOW REPLICA STATUS（8.0.22+）。
// 不是从库时返回 nil，不算错误。
func collectReplication(ctx context.Context, db *sql.DB) (*model.ReplicationStatus, error) {
	rows, err := db.QueryContext(ctx, "SHOW SLAVE STATUS")
	if err != nil {
		if !isParseError(err) {
			return nil, fmt.Errorf("执行 SHOW SLAVE STATUS: %w", err)
		}
		rows, err = db.QueryContext(ctx, "SHOW REPLICA STATUS")
		if err != nil {
			return nil, fmt.Errorf("执行 SHOW REPLICA STATUS: %w", err)
		}
	}
	defer rows.Close()

	raw, err := scanFirstRow(rows)
	if err != nil {
		return nil, fmt.Errorf("读取复制状态: %w", err)
	}
	if raw == nil {
		return nil, nil // 非从库
	}
	return &model.ReplicationStatus{
		Role:                "slave",
		MasterHost:          raw["Master_Host"],
		MasterPort:          raw["Master_Port"],
		IORunning:           raw["Slave_IO_Running"],
		SQLRunning:          raw["Slave_SQL_Running"],
		SecondsBehindMaster: raw["Seconds_Behind_Master"],
		LastIOError:         raw["Last_IO_Error"],
		LastSQLError:        raw["Last_SQL_Error"],
		Raw:                 raw,
	}, nil
}

// collectAutoIncrement 采集业务表自增值，按自增值降序返回前 limit 张。
// auto_increment 为 NULL 的表（无自增列）直接排除；
// limit 服务端截断，避免大实例（数万张表）全量拉取。
func collectAutoIncrement(ctx context.Context, db *sql.DB, limit int) ([]model.TableAutoIncrement, error) {
	if limit <= 0 {
		limit = 100
	}
	query := "SELECT table_schema, table_name, engine, auto_increment " +
		"FROM information_schema.tables " +
		"WHERE table_schema NOT IN " + systemSchemas + " AND auto_increment IS NOT NULL " +
		"ORDER BY auto_increment DESC LIMIT ?"
	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询 information_schema.tables: %w", err)
	}
	defer rows.Close()

	var out []model.TableAutoIncrement
	for rows.Next() {
		var t model.TableAutoIncrement
		var autoInc sql.NullInt64
		if err := rows.Scan(&t.Schema, &t.Table, &t.Engine, &autoInc); err != nil {
			return nil, fmt.Errorf("读取自增表行: %w", err)
		}
		if autoInc.Valid {
			t.AutoIncrement = &autoInc.Int64
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// scanFirstRow 动态扫描第一行为 列名->值 的 map；无数据时返回 nil。
// SHOW SLAVE STATUS 的列集随版本差异很大，不能静态 Scan。
func scanFirstRow(rows *sql.Rows) (map[string]string, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("读取列信息: %w", err)
	}
	if !rows.Next() {
		return nil, rows.Err()
	}

	// sql.RawBytes + 拷贝，避免 Scan 目标复用导致的数据错乱
	vals := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, fmt.Errorf("扫描行: %w", err)
	}

	out := make(map[string]string, len(cols))
	for i, c := range cols {
		if vals[i] == nil {
			out[c] = ""
		} else {
			out[c] = string(vals[i])
		}
	}
	return out, nil
}

// computeDelta 计算两次采样间各计数器的差值。
// 新值小于旧值说明计数器被重置（实例重启 / FLUSH STATUS），此时取新值。
func computeDelta(before, after map[string]string) map[string]int64 {
	delta := make(map[string]int64, len(statusDeltaKeys))
	for _, k := range statusDeltaKeys {
		b, okB := parseInt(before[k])
		a, okA := parseInt(after[k])
		if !okB || !okA {
			continue
		}
		if a < b {
			delta[k] = a
		} else {
			delta[k] = a - b
		}
	}
	return delta
}

func filterKeys(m map[string]string, keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

func parseInt(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func waitDuration(seconds int) <-chan time.Time {
	return time.After(time.Duration(seconds) * time.Second)
}
