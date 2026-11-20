package collector

// variableKeys 采集 SHOW GLOBAL VARIABLES 时关注的变量名（保持巡检报告的展示顺序）。
// 与旧版 lib/innodb_con.py 的采集清单一致；缺失的变量会被跳过而不是报错，
// 以兼容不同版本的 MySQL。
var variableKeys = []string{
	// 基础信息
	"server_id", "port", "datadir", "log_error", "sql_mode",
	// 复制与日志
	"log_bin", "binlog_format", "read_only", "log_slave_updates",
	"slave_skip_errors", "expire_logs_days", "binlog_expire_logs_seconds",
	"gtid_mode", "enforce_gtid_consistency",
	"master_info_repository", "relay_log_info_repository", "relay_log_recovery",
	// 双一参数（可靠性）
	"innodb_flush_log_at_trx_commit", "sync_binlog",
	// InnoDB
	"innodb_buffer_pool_size", "innodb_log_file_size",
	"innodb_io_capacity", "innodb_max_dirty_pages_pct",
	"innodb_file_per_table", "innodb_file_format_max",
	"innodb_lock_wait_timeout", "innodb_rollback_on_timeout",
	// 缓存与内存
	"query_cache_type", "query_cache_size",
	"sort_buffer_size", "join_buffer_size", "thread_cache_size",
	"key_buffer_size", "read_buffer_size", "read_rnd_buffer_size",
	// 连接与超时
	"max_connections", "max_connect_errors", "open_files_limit",
	"interactive_timeout", "wait_timeout", "table_open_cache",
	// 慢查询
	"slow_query_log_file", "log_queries_not_using_indexes",
}

// statusDeltaKeys 两次采样求差值的计数器（反映采样间隔内的流量/操作）。
var statusDeltaKeys = []string{
	"Com_insert", "Com_select", "Com_update", "Com_delete", "Com_replace",
	"Com_commit", "Com_rollback",
	"Questions", "Queries",
	"Innodb_rows_inserted", "Innodb_rows_read", "Innodb_rows_updated", "Innodb_rows_deleted",
	"Qcache_hits",
}

// statusRawKeys 直接取累计值的计数器（等待/连接等瞬时或累计状态）。
var statusRawKeys = []string{
	"Innodb_row_lock_waits", "Innodb_log_waits",
	"Innodb_row_lock_current_waits", "Innodb_row_lock_time",
	"Table_locks_waited",
	"Open_files", "Open_tables",
	"Threads_created", "Threads_running", "Threads_connected",
	"Aborted_clients", "Aborted_connects",
	"Connections", "Max_used_connections",
}

// systemSchemas 巡检时排除的系统库。
var systemSchemas = "('information_schema','performance_schema','mysql','sys')"
