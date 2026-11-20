// Package report 负责把巡检结果渲染为人类可读的终端文本报告：
// 顶部汇总风险检查项，主体按语义分组展示参数与状态，并使用 tabwriter 对齐、
// 对终端输出启用 ANSI 配色（NO_COLOR 环境变量或非终端输出时自动降级为纯文本）。
package report

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/enmotplinux/onsite-inspection/internal/model"
)

// ── 终端配色 ────────────────────────────────────────────────────────────────

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// stylist 依据输出目标是否为彩色终端决定是否包裹 ANSI 转义码，
// 保证重定向到文件或管道时输出仍是干净的纯文本。
type stylist struct{ color bool }

func newStylist(w io.Writer) stylist {
	if os.Getenv("NO_COLOR") != "" {
		return stylist{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return stylist{}
	}
	return stylist{color: term.IsTerminal(int(f.Fd()))}
}

func (s stylist) wrap(code, text string) string {
	if !s.color {
		return text
	}
	return code + text + ansiReset
}

func (s stylist) bold(t string) string   { return s.wrap(ansiBold, t) }
func (s stylist) red(t string) string    { return s.wrap(ansiRed, t) }
func (s stylist) green(t string) string  { return s.wrap(ansiGreen, t) }
func (s stylist) yellow(t string) string { return s.wrap(ansiYellow, t) }
func (s stylist) cyan(t string) string   { return s.wrap(ansiCyan, t) }

// ── 风险检查 ────────────────────────────────────────────────────────────────

// finding 一条巡检发现：risk=true 为高风险（红色），否则为警告（黄色）。
type finding struct {
	risk   bool
	item   string
	detail string
}

// checkFindings 对巡检结果做规则检查：双一参数、复制健康、连接使用率、自增溢出。
func checkFindings(r *model.InspectionResult) []finding {
	var out []finding
	v := r.Variables

	// 双一参数（宕机丢数据风险）
	if s := v["sync_binlog"]; s != "" && s != "1" {
		out = append(out, finding{risk: true, item: "sync_binlog=" + s,
			detail: "双一参数不满足，宕机可能丢失 binlog 事件"})
	}
	if s := v["innodb_flush_log_at_trx_commit"]; s != "" && s != "1" {
		out = append(out, finding{risk: true, item: "innodb_flush_log_at_trx_commit=" + s,
			detail: "双一参数不满足，宕机可能丢失事务"})
	}

	// 复制健康
	if rep := r.Replication; rep != nil {
		if strings.EqualFold(rep.IORunning, "no") || strings.EqualFold(rep.SQLRunning, "no") {
			out = append(out, finding{risk: true, item: "复制线程未运行",
				detail: fmt.Sprintf("Slave_IO_Running=%s, Slave_SQL_Running=%s", rep.IORunning, rep.SQLRunning)})
		}
		if d, err := strconv.Atoi(rep.SecondsBehindMaster); err == nil && d > 60 {
			out = append(out, finding{item: fmt.Sprintf("Seconds_Behind_Master=%d", d),
				detail: "复制延迟超过 60 秒"})
		}
	}

	// 连接使用率
	maxUsed, err1 := strconv.ParseInt(r.StatusRaw["Max_used_connections"], 10, 64)
	maxConn, err2 := strconv.ParseInt(v["max_connections"], 10, 64)
	if err1 == nil && err2 == nil && maxConn > 0 {
		if pct := maxUsed * 100 / maxConn; pct >= 80 {
			out = append(out, finding{item: fmt.Sprintf("连接使用率 %d%%（%d/%d）", pct, maxUsed, maxConn),
				detail: "历史最大连接数已接近 max_connections"})
		}
	}

	// 自增值接近有符号 INT 上限（2147483647 的 90%）
	const intNearOverflow = int64(1932735282)
	for _, t := range r.AutoIncrement {
		if t.AutoIncrement != nil && *t.AutoIncrement > intNearOverflow {
			out = append(out, finding{
				item:  fmt.Sprintf("%s.%s AUTO_INCREMENT=%d", t.Schema, t.Table, *t.AutoIncrement),
				detail: "自增值已超过 19 亿，若列为有符号 INT 已临近溢出，请确认列类型并改为 BIGINT"})
		}
	}
	return out
}

// ── 主渲染 ──────────────────────────────────────────────────────────────────

const (
	lineFull  = "================================================================================"
	lineThin  = "--------------------------------------------------------------------------------"
)

// variableGroups 报告的参数分组（保持语义顺序）。
var variableGroups = []struct {
	title string
	keys  []string
}{
	{"基础信息", []string{"server_id", "port", "datadir", "sql_mode", "log_error"}},
	{"日志与复制", []string{
		"log_bin", "binlog_format", "read_only", "log_slave_updates", "slave_skip_errors",
		"expire_logs_days", "binlog_expire_logs_seconds", "gtid_mode", "enforce_gtid_consistency",
		"master_info_repository", "relay_log_info_repository", "relay_log_recovery"}},
	{"双一参数（数据可靠性）", []string{"innodb_flush_log_at_trx_commit", "sync_binlog"}},
	{"InnoDB", []string{
		"innodb_buffer_pool_size", "innodb_log_file_size", "innodb_io_capacity",
		"innodb_max_dirty_pages_pct", "innodb_file_per_table", "innodb_file_format_max",
		"innodb_lock_wait_timeout", "innodb_rollback_on_timeout"}},
	{"缓存与内存", []string{
		"query_cache_type", "query_cache_size", "sort_buffer_size", "join_buffer_size",
		"thread_cache_size", "key_buffer_size", "read_buffer_size", "read_rnd_buffer_size"}},
	{"连接与超时", []string{
		"max_connections", "max_connect_errors", "open_files_limit", "table_open_cache",
		"interactive_timeout", "wait_timeout"}},
	{"慢查询", []string{"slow_query_log_file", "log_queries_not_using_indexes"}},
}

// byteKeys 值为字节数的参数，渲染时附加人类可读单位。
var byteKeys = map[string]bool{
	"innodb_buffer_pool_size": true,
	"innodb_log_file_size":    true,
	"key_buffer_size":         true,
	"sort_buffer_size":        true,
	"join_buffer_size":        true,
	"read_buffer_size":        true,
	"read_rnd_buffer_size":    true,
	"query_cache_size":        true,
}

// Render 将一次巡检结果渲染为分组文本报告。
// autoIncrementTopN 控制自增使用情况展示的表数量上限（<=0 表示展示全部）。
func Render(w io.Writer, r *model.InspectionResult, autoIncrementTopN int) {
	s := newStylist(w)
	renderHeader(w, s, r)

	// 采集失败时只展示错误，不再输出误导性的"未发现风险"
	if r.Error != "" {
		fmt.Fprintf(w, "  %s %s\n\n", s.red("采集异常:"), r.Error)
		return
	}

	renderFindings(w, s, checkFindings(r))
	if len(r.Variables) > 0 {
		renderVariables(w, s, r.Variables)
	}
	if len(r.StatusDelta) > 0 || len(r.StatusRaw) > 0 {
		renderStatus(w, s, r)
	}
	renderReplication(w, s, r.Replication)
	renderAutoIncrement(w, s, r.AutoIncrement, autoIncrementTopN)
}

func renderHeader(w io.Writer, s stylist, r *model.InspectionResult) {
	fmt.Fprintf(w, "%s\n", s.bold(lineFull))
	fmt.Fprintf(w, " %s\n", s.bold("MySQL 巡检报告"))
	fmt.Fprintf(w, "%s\n", s.bold(lineFull))
	fmt.Fprintf(w, "  实例  %s (%s)\n", s.bold(r.InstanceID), hostPort(r))
	fmt.Fprintf(w, "  采集  %s    耗时 %s    状态采样间隔 %d s\n",
		r.CollectedAt.Local().Format("2006-01-02 15:04:05"),
		fmt.Sprintf("%.2fs", float64(r.DurationMS)/1000),
		r.SampleSeconds)
	fmt.Fprintf(w, "%s\n", s.bold(lineFull))
}

func hostPort(r *model.InspectionResult) string {
	if r.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

func renderFindings(w io.Writer, s stylist, findings []finding) {
	fmt.Fprintf(w, " %s\n", s.bold("[风险检查]"))
	if len(findings) == 0 {
		fmt.Fprintf(w, "   %s 未发现风险项\n", s.green("[OK]"))
		return
	}
	for _, f := range findings {
		mark := s.yellow("[WARN]")
		if f.risk {
			mark = s.red("[RISK]")
		}
		fmt.Fprintf(w, "   %s %s\n        %s\n", mark, f.item, f.detail)
	}
	fmt.Fprintf(w, "\n")
}

// renderVariables 按语义分组展示关键参数，字节数自动附上 K/M/G 单位。
func renderVariables(w io.Writer, s stylist, v map[string]string) {
	fmt.Fprintf(w, " %s\n", s.bold("[关键参数]"))
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, g := range variableGroups {
		fmt.Fprintf(tw, "  %s\n", s.cyan(g.title))
		for _, k := range g.keys {
			if val, ok := v[k]; ok {
				if byteKeys[k] {
					val = humanBytes(val)
				}
				fmt.Fprintf(tw, "    %s\t%s\n", k, val)
			}
		}
	}
	tw.Flush()
	fmt.Fprintf(w, "\n")
}

func renderStatus(w io.Writer, s stylist, r *model.InspectionResult) {
	d := r.StatusDelta
	seconds := int64(r.SampleSeconds)
	if seconds <= 0 {
		seconds = 1
	}

	fmt.Fprintf(w, " %s（%d 秒内增量 -> 均值/秒）\n", s.bold("[流量与操作]"), r.SampleSeconds)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	rows := [][2]string{
		{"SELECT", "Com_select"},
		{"INSERT", "Com_insert"},
		{"UPDATE", "Com_update"},
		{"DELETE", "Com_delete"},
		{"REPLACE", "Com_replace"},
		{"COMMIT", "Com_commit"},
		{"ROLLBACK", "Com_rollback"},
		{"Questions", "Questions"},
		{"Queries", "Queries"},
		{"Innodb_rows_read", "Innodb_rows_read"},
		{"Innodb_rows_inserted", "Innodb_rows_inserted"},
		{"Innodb_rows_updated", "Innodb_rows_updated"},
		{"Innodb_rows_deleted", "Innodb_rows_deleted"},
		{"Qcache_hits", "Qcache_hits"},
	}
	for _, row := range rows {
		if v, ok := d[row[1]]; ok {
			fmt.Fprintf(tw, "  %-20s\t%12d\t(%d/s)\n", row[0], v, v/seconds)
		}
	}
	tw.Flush()

	raw := r.StatusRaw
	if len(raw) > 0 {
		fmt.Fprintf(w, " %s（累计值）\n", s.bold("[等待与并发]"))
		tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, k := range []string{
			"Innodb_row_lock_waits", "Innodb_row_lock_time", "Innodb_row_lock_current_waits",
			"Innodb_log_waits", "Table_locks_waited",
		} {
			if v, ok := raw[k]; ok {
				fmt.Fprintf(tw, "  %-36s\t%s\n", k, v)
			}
		}
		tw.Flush()

		fmt.Fprintf(w, " %s（累计值）\n", s.bold("[连接与文件]"))
		tw = tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, k := range []string{
			"Threads_running", "Threads_connected", "Threads_created",
			"Connections", "Max_used_connections",
			"Aborted_clients", "Aborted_connects",
			"Open_files", "Open_tables",
		} {
			if v, ok := raw[k]; ok {
				fmt.Fprintf(tw, "  %-36s\t%s\n", k, v)
			}
		}
		tw.Flush()
	}
	fmt.Fprintf(w, "\n")
}

// renderReplication 展示复制状态，线程状态与健康程度着色。
func renderReplication(w io.Writer, s stylist, rep *model.ReplicationStatus) {
	fmt.Fprintf(w, " %s\n", s.bold("[复制状态]"))
	if rep == nil {
		fmt.Fprintf(w, "  非 slave / replica，跳过\n\n")
		return
	}
	fmt.Fprintf(w, "  Master: %s:%s\n", rep.MasterHost, rep.MasterPort)
	fmt.Fprintf(w, "  %-22s %s\n", "Slave_IO_Running:", runningBadge(s, rep.IORunning))
	fmt.Fprintf(w, "  %-22s %s\n", "Slave_SQL_Running:", runningBadge(s, rep.SQLRunning))
	fmt.Fprintf(w, "  %-22s %s\n", "Seconds_Behind_Master:", rep.SecondsBehindMaster)
	fmt.Fprintf(w, "  %-22s %s\n", "Last_IO_Error:", orDash(rep.LastIOError))
	fmt.Fprintf(w, "  %-22s %s\n", "Last_SQL_Error:", orDash(rep.LastSQLError))
	fmt.Fprintf(w, "\n")
}

func runningBadge(s stylist, state string) string {
	switch strings.ToLower(state) {
	case "yes":
		return s.green("Yes")
	case "no":
		return s.red("No")
	case "connecting":
		return s.yellow("Connecting")
	default:
		return state
	}
}

// renderAutoIncrement 展示自增值最大的业务表（旧版采集了却从不展示）。
func renderAutoIncrement(w io.Writer, s stylist, tables []model.TableAutoIncrement, topN int) {
	fmt.Fprintf(w, " %s\n", s.bold("[业务表自增 ID 使用情况]"))
	if len(tables) == 0 {
		fmt.Fprintf(w, "  未发现含自增列的业务表\n\n")
		return
	}
	if topN > 0 && len(tables) > topN {
		total := len(tables)
		tables = tables[:topN]
		fmt.Fprintf(w, "  共 %d 张含自增列业务表，按 AUTO_INCREMENT 降序展示前 %d 张\n", total, topN)
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s\t%s\t%s\n", "表", "引擎", "AUTO_INCREMENT")
	for _, t := range tables {
		auto := ""
		if t.AutoIncrement != nil {
			auto = strconv.FormatInt(*t.AutoIncrement, 10)
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", t.Schema+"."+t.Table, t.Engine, auto)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n")
}

// ── 工具函数 ────────────────────────────────────────────────────────────────

// humanBytes 为字节数值附加人类可读单位，如 "134217728 (128M)"、"1073741824 (1G)"。
func humanBytes(raw string) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < KB {
		return raw
	}
	switch {
	case v >= GB:
		return fmt.Sprintf("%s (%sG)", raw, trimFloat(float64(v)/GB, 2))
	case v >= MB:
		return fmt.Sprintf("%s (%sM)", raw, trimFloat(float64(v)/MB, 1))
	default:
		return fmt.Sprintf("%s (%sK)", raw, trimFloat(float64(v)/KB, 1))
	}
}

// trimFloat 定点格式化并去掉尾部多余的零（128.0 -> 128, 128.50 -> 128.5）。
func trimFloat(f float64, prec int) string {
	s := strconv.FormatFloat(f, 'f', prec, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// RenderSummaryLine 渲染单条实例的执行摘要（用于 run 命令实时输出）。
func RenderSummaryLine(w io.Writer, sum model.Summary) {
	s := newStylist(w)
	status := s.green("[ OK ]")
	if sum.Err != nil {
		status = s.red("[FAIL]")
	}
	fmt.Fprintf(w, "%s %-32s %10s\n", status, sum.InstanceID, sum.Duration.Round(time.Millisecond))
	if sum.Err != nil {
		fmt.Fprintf(w, "       %v\n", sum.Err)
	}
}
