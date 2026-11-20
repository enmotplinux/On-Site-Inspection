package report

import (
	"bytes"
	"testing"
	"time"

	"github.com/enmotplinux/onsite-inspection/internal/model"
)

func sampleResult() *model.InspectionResult {
	auto := int64(12345)
	return &model.InspectionResult{
		InstanceID:    "192.168.1.10_3306",
		Host:          "192.168.1.10",
		Port:          3306,
		CollectedAt:   time.Date(2026, 11, 20, 14, 0, 0, 0, time.Local),
		SampleSeconds: 1,
		Variables: map[string]string{
			"server_id": "1", "sync_binlog": "1",
			"innodb_flush_log_at_trx_commit": "1",
			"innodb_buffer_pool_size":        "134217728", // 128M
		},
		StatusDelta: map[string]int64{"Com_select": 100, "Com_insert": 10},
		StatusRaw: map[string]string{
			"Threads_running": "2", "Innodb_row_lock_waits": "5",
		},
		Replication: &model.ReplicationStatus{
			Role: "slave", MasterHost: "192.168.1.1", MasterPort: "3306",
			IORunning: "Yes", SQLRunning: "Yes", SecondsBehindMaster: "0",
		},
		AutoIncrement: []model.TableAutoIncrement{
			{Schema: "biz", Table: "orders", Engine: "InnoDB", AutoIncrement: &auto},
		},
	}
}

func TestRenderFullResult(t *testing.T) {
	var buf bytes.Buffer
	Render(&buf, sampleResult(), 10)
	out := buf.String()

	// 各分组的关键内容都应出现
	for _, want := range []string{
		"192.168.1.10_3306",
		"[OK] 未发现风险项", // 双一参数满足、复制正常
		"innodb_buffer_pool_size", "(128M)",
		"SELECT", "100/s",
		"Innodb_row_lock_waits", "Threads_running",
		"Slave_IO_Running", "Seconds_Behind_Master",
		"biz.orders",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("报告应包含 %q\n完整输出:\n%s", want, out)
		}
	}
}

func TestRenderErrorResult(t *testing.T) {
	r := &model.InspectionResult{
		InstanceID:  "bad_host_3306",
		CollectedAt: time.Now(),
		Error:       "dial tcp 1.2.3.4:3306: connection refused",
	}
	var buf bytes.Buffer
	Render(&buf, r, 10) // 不应 panic
	if !bytes.Contains(buf.Bytes(), []byte("connection refused")) {
		t.Errorf("采集异常应渲染在报告中，实际输出:\n%s", buf.String())
	}
	// 失败报告不应再输出"未发现风险项"造成误导
	if bytes.Contains(buf.Bytes(), []byte("未发现风险项")) {
		t.Error("采集失败的报告不应输出风险检查结论")
	}
}

func TestRenderNoReplication(t *testing.T) {
	r := &model.InspectionResult{
		InstanceID:  "master_3306",
		CollectedAt: time.Now(),
		Variables:   map[string]string{"server_id": "100"},
	}
	var buf bytes.Buffer
	Render(&buf, r, 10)
	if !bytes.Contains(buf.Bytes(), []byte("非 slave")) {
		t.Errorf("非从库应有提示，实际输出:\n%s", buf.String())
	}
}

func TestRenderFindings(t *testing.T) {
	auto := int64(2000000000) // 20 亿，超过 INT 上限 90%
	r := sampleResult()
	r.Variables["sync_binlog"] = "0"
	r.Variables["innodb_flush_log_at_trx_commit"] = "2"
	r.Replication.IORunning = "No"
	r.Replication.SecondsBehindMaster = "120"
	r.StatusRaw["Max_used_connections"] = "90"
	r.Variables["max_connections"] = "100"
	r.AutoIncrement = []model.TableAutoIncrement{
		{Schema: "biz", Table: "huge", Engine: "InnoDB", AutoIncrement: &auto},
	}

	var buf bytes.Buffer
	Render(&buf, r, 10)
	out := buf.String()

	for _, want := range []string{
		"[RISK] sync_binlog=0",
		"[RISK] innodb_flush_log_at_trx_commit=2",
		"[RISK] 复制线程未运行",
		"[WARN] Seconds_Behind_Master=120",
		"[WARN] 连接使用率 90%（90/100）",
		"[WARN] biz.huge AUTO_INCREMENT=2000000000",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("风险检查应包含 %q\n完整输出:\n%s", want, out)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"134217728", "134217728 (128M)"},          // 128M
		{"1073741824", "1073741824 (1G)"},          // 1G
		{"1610612736", "1610612736 (1.5G)"},        // 1.5G
		{"1024", "1024 (1K)"},                      // 1K
		{"512", "512"},                            // 小于 1K 不转换
		{"abc", "abc"},                             // 非数字不转换
		{"", ""},                                  // 空值原样返回
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
