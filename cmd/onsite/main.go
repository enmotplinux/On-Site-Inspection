// onsite 是 MySQL 现场巡检工具的命令行入口。
//
// 子命令:
//
//	onsite add                       录入待巡检的 MySQL 实例
//	onsite list                      列出全部待巡检实例
//	onsite remove -id <host_port>    删除实例
//	onsite run                       并发巡检全部实例并存档
//	onsite report [-id <host_port>]  查看最近一次巡检报告
//
// 全局配置: -config <path> 指定配置文件（默认按 conf/config.json 多路径查找）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"golang.org/x/term"

	"github.com/enmotplinux/onsite-inspection/internal/collector"
	"github.com/enmotplinux/onsite-inspection/internal/config"
	"github.com/enmotplinux/onsite-inspection/internal/model"
	"github.com/enmotplinux/onsite-inspection/internal/report"
	"github.com/enmotplinux/onsite-inspection/internal/storage"
)

const usage = `用法: onsite <子命令> [参数]

子命令:
  add      录入待巡检的 MySQL 实例
             onsite add -host 192.168.1.10 -port 3306 -user root [-password xxx]
             密码优先级: -password > 环境变量 ONSITE_MYSQL_PASSWORD > 终端隐藏输入
  list     列出全部待巡检实例
  remove   删除实例: onsite remove -id 192.168.1.10_3306
  run      并发巡检全部实例，结果存入 MongoDB
  report   查看巡检报告: onsite report [-id 192.168.1.10_3306]（省略 -id 则输出全部实例）

全局参数:
  -config string   配置文件路径（默认依次查找 ONSITE_CONFIG、./conf/config.json 等）

环境变量:
  ONSITE_CONFIG           指定配置文件路径
  ONSITE_MONGO_PASSWORD   覆盖配置文件中的 MongoDB 密码（避免明文落盘）
  ONSITE_MYSQL_PASSWORD   add 录入实例时的 MySQL 密码（避免终端交互）
  NO_COLOR                禁用报告的 ANSI 颜色输出
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// Ctrl-C / kill 时优雅取消进行中的采集
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var err error
	switch os.Args[1] {
	case "add":
		err = runAdd(ctx, os.Args[2:])
	case "list":
		err = runList(ctx, os.Args[2:])
	case "remove":
		err = runRemove(ctx, os.Args[2:])
	case "run":
		err = runRun(ctx, os.Args[2:])
	case "report":
		err = runReport(ctx, os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

// app 聚合一次命令执行所需的资源：配置 + 存储连接。
type app struct {
	cfg   config.Config
	store storage.Store
	close func()
}

// newApp 解析 -config 参数，加载配置并建立存储连接。
func newApp(ctx context.Context, args []string) (*app, error) {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	configPath := fs.String("config", "", "配置文件路径")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	path, err := config.ResolvePath(*configPath)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}

	store, err := storage.NewMongoStore(ctx, cfg.MongoDB)
	if err != nil {
		return nil, err
	}
	closeFn := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Collect.ConnectTimeout())
		defer cancel()
		_ = store.Close(shutdownCtx)
	}
	return &app{cfg: cfg, store: store, close: closeFn}, nil
}

func runAdd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	host := fs.String("host", "", "MySQL 主机地址")
	port := fs.Int("port", 3306, "MySQL 端口")
	user := fs.String("user", "root", "MySQL 用户")
	password := fs.String("password", "", "MySQL 密码（为空时读取环境变量或终端输入）")
	_ = fs.Parse(args)

	if *host == "" {
		return fmt.Errorf("必须通过 -host 指定 MySQL 主机地址")
	}

	// 先连通 MongoDB 再交互输密码：避免库配置错误时用户白输一遍密码
	a, err := newApp(ctx, fs.Args())
	if err != nil {
		return err
	}
	defer a.close()

	// 密码优先级: 参数 > 环境变量 > 终端隐藏输入（避免回显泄露）
	if *password == "" {
		if v := os.Getenv("ONSITE_MYSQL_PASSWORD"); v != "" {
			*password = v
		} else if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Printf("password for %s@%s:%d: ", *user, *host, *port)
			b, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("读取密码: %w", err)
			}
			*password = string(b)
		}
	}

	inst := model.Instance{
		ID:       *host + "_" + strconv.Itoa(*port),
		Host:     *host,
		Port:     *port,
		User:     *user,
		Password: *password,
	}
	if err := a.store.AddInstance(ctx, inst); err != nil {
		return err
	}
	fmt.Printf("已录入实例 %s\n", inst.ID)
	return nil
}

func runList(ctx context.Context, args []string) error {
	a, err := newApp(ctx, args)
	if err != nil {
		return err
	}
	defer a.close()

	instances, err := a.store.ListInstances(ctx)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		fmt.Println("暂无实例，请先用 onsite add 录入")
		return nil
	}
	fmt.Printf("%-32s %-8s %-12s %s\n", "ID (host_port)", "端口", "用户", "录入时间")
	for _, inst := range instances {
		fmt.Printf("%-32s %-8d %-12s %s\n",
			inst.ID, inst.Port, inst.User, inst.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	}
	return nil
}

func runRemove(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	id := fs.String("id", "", "实例 ID（host_port）")
	_ = fs.Parse(args)
	if *id == "" {
		return fmt.Errorf("必须通过 -id 指定要删除的实例")
	}

	a, err := newApp(ctx, fs.Args())
	if err != nil {
		return err
	}
	defer a.close()

	removed, err := a.store.RemoveInstance(ctx, *id)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("实例 %s 不存在", *id)
	}
	fmt.Printf("已删除实例 %s\n", *id)
	return nil
}

func runRun(ctx context.Context, args []string) error {
	a, err := newApp(ctx, args)
	if err != nil {
		return err
	}
	defer a.close()

	// 实时进度：每完成一台立即输出一行摘要（channel 串行化，避免行交错）
	progress := make(chan model.Summary)
	drained := make(chan struct{})
	go func() {
		for sum := range progress {
			report.RenderSummaryLine(os.Stdout, sum)
		}
		close(drained)
	}()

	summaries, err := collector.New(a.store, a.cfg).Run(ctx, func(sum model.Summary) {
		progress <- sum
	})
	close(progress)
	<-drained
	if err != nil {
		return err
	}

	// 汇总
	failed := 0
	for _, s := range summaries {
		if s.Err != nil {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("巡检完成: %d 成功 / %d 失败", len(summaries)-failed, failed)
	}
	fmt.Printf("巡检完成: %d 个实例全部成功\n", len(summaries))
	return nil
}

func runReport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	id := fs.String("id", "", "实例 ID（省略则输出全部实例）")
	_ = fs.Parse(args)

	a, err := newApp(ctx, fs.Args())
	if err != nil {
		return err
	}
	defer a.close()

	targets := []*string{id}
	if *id == "" {
		instances, err := a.store.ListInstances(ctx)
		if err != nil {
			return err
		}
		if len(instances) == 0 {
			fmt.Println("暂无实例，请先用 onsite add 录入")
			return nil
		}
		targets = make([]*string, 0, len(instances))
		for i := range instances {
			targets = append(targets, &instances[i].ID)
		}
	}

	first := true
	missing := []string{}
	for _, t := range targets {
		result, err := a.store.LatestResult(ctx, *t)
		if err != nil {
			return err
		}
		if result == nil {
			missing = append(missing, *t)
			continue
		}
		if !first {
			fmt.Println()
		}
		first = false
		report.Render(os.Stdout, result, a.cfg.Collect.AutoIncrementTopN)
	}
	for _, m := range missing {
		fmt.Fprintf(os.Stderr, "实例 %s 尚无巡检结果，请先执行 onsite run\n", m)
	}
	return nil
}
