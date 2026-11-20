package collector

import (
	"reflect"
	"testing"
)

func TestComputeDelta(t *testing.T) {
	tests := []struct {
		name   string
		before map[string]string
		after  map[string]string
		want   map[string]int64
	}{
		{
			name: "正常差值",
			before: map[string]string{
				"Com_insert": "100", "Com_select": "1000", "Questions": "5000",
			},
			after: map[string]string{
				"Com_insert": "135", "Com_select": "1240", "Questions": "5480",
			},
			want: map[string]int64{
				"Com_insert": 35, "Com_select": 240, "Questions": 480,
			},
		},
		{
			name: "计数器回绕（实例重启/FLUSH STATUS 后新值更小，取新值）",
			before: map[string]string{
				"Com_insert": "9999", "Com_select": "9999",
			},
			after: map[string]string{
				"Com_insert": "3", "Com_select": "5",
			},
			want: map[string]int64{
				"Com_insert": 3, "Com_select": 5,
			},
		},
		{
			name: "缺失的 key 跳过（不同版本变量差异）",
			before: map[string]string{
				"Com_insert": "10", "Qcache_hits": "1",
			},
			after: map[string]string{
				"Com_insert": "20", // Qcache_hits 缺失（如 8.0 已移除 query cache）
			},
			want: map[string]int64{
				"Com_insert": 10,
			},
		},
		{
			name:   "空输入",
			before: map[string]string{},
			after:  map[string]string{},
			want:   map[string]int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDelta(tt.before, tt.after)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("computeDelta() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterKeys(t *testing.T) {
	in := map[string]string{
		"Threads_running": "2", "Com_insert": "10", "NotWatched": "x",
	}
	got := filterKeys(in, []string{"Threads_running", "Com_insert", "Missing"})
	want := map[string]string{"Threads_running": "2", "Com_insert": "10"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterKeys() = %v, want %v", got, want)
	}
}

func TestCollectVariablesKeyCoverage(t *testing.T) {
	// 旧版 Python 采集的关键变量必须全部保留在采集清单中
	mustHave := []string{
		"innodb_buffer_pool_size", "sync_binlog", "binlog_format",
		"innodb_flush_log_at_trx_commit", "read_only", "innodb_log_file_size",
		"log_slave_updates", "query_cache_type", "innodb_file_per_table",
		"max_connections", "max_connect_errors", "table_open_cache",
		"slave_skip_errors", "datadir", "open_files_limit", "sort_buffer_size",
		"join_buffer_size", "thread_cache_size", "interactive_timeout",
		"wait_timeout", "log_error", "slow_query_log_file", "server_id",
		"log_bin", "gtid_mode", "enforce_gtid_consistency", "key_buffer_size",
		"read_buffer_size", "read_rnd_buffer_size", "innodb_lock_wait_timeout",
		"innodb_rollback_on_timeout", "sql_mode",
	}
	set := make(map[string]bool, len(variableKeys))
	for _, k := range variableKeys {
		set[k] = true
	}
	for _, k := range mustHave {
		if !set[k] {
			t.Errorf("采集清单缺少旧版关注的变量: %s", k)
		}
	}
}
