package collector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/enmotplinux/onsite-inspection/internal/config"
	"github.com/enmotplinux/onsite-inspection/internal/model"
	"github.com/enmotplinux/onsite-inspection/internal/storage"
)

// Collector 巡检调度器：并发采集全部目标实例并把结果写入存储。
type Collector struct {
	store storage.Store
	cfg   config.Config
}

// New 创建调度器。
func New(store storage.Store, cfg config.Config) *Collector {
	return &Collector{store: store, cfg: cfg}
}

// Run 并发采集全部实例，返回每个实例的执行摘要。
// onDone 在每个实例完成时被立即调用（用于实时进度输出）；可为 nil。
// 单实例失败只影响自身，不会中断其他实例。
func (c *Collector) Run(ctx context.Context, onDone func(model.Summary)) ([]model.Summary, error) {
	if onDone == nil {
		onDone = func(model.Summary) {}
	}

	instances, err := c.store.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可巡检的实例，请先用 add 子命令录入")
	}

	summaries := make([]model.Summary, len(instances))
	sem := make(chan struct{}, c.cfg.Collect.Concurrency)
	var wg sync.WaitGroup

	for i, inst := range instances {
		wg.Add(1)
		go func(idx int, inst model.Instance) {
			defer wg.Done()
			start := time.Now()

			// panic 防护：单实例内部异常不应拖垮整个巡检进程
			defer func() {
				if p := recover(); p != nil {
					summaries[idx] = model.Summary{
						InstanceID: inst.ID,
						Duration:   time.Since(start),
						Err:        fmt.Errorf("采集协程 panic: %v", p),
					}
					onDone(summaries[idx])
				}
			}()

			// 信号量控制并发数；ctx 取消时不再启动新任务
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				summaries[idx] = model.Summary{InstanceID: inst.ID, Err: ctx.Err()}
				onDone(summaries[idx])
				return
			}

			err := c.collectAndSave(ctx, inst)
			summaries[idx] = model.Summary{
				InstanceID: inst.ID,
				Duration:   time.Since(start),
				Err:        err,
			}
			onDone(summaries[idx])
		}(i, inst)
	}
	wg.Wait()
	return summaries, nil
}

// collectAndSave 采集单个实例并落库。采集失败也会落库一份带 Error 的结果，
// 便于事后通过 report 查看失败原因。
func (c *Collector) collectAndSave(ctx context.Context, inst model.Instance) error {
	result := c.CollectInstance(ctx, inst)
	if err := c.store.SaveResult(ctx, result, c.cfg.Collect.ResultRetention); err != nil {
		return fmt.Errorf("保存结果失败: %w", err)
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

// CollectInstance 采集单个实例，三层并行：
//  1. 多实例之间由 Run 调度并发；
//  2. 实例内部，状态采样的等待间隔里并行抓取复制状态与自增信息；
//  3. 任一步骤出错记入 result.Error 并保留已成功采集的部分（尽力而为），
//     替代旧版"一崩全崩"的行为。
func (c *Collector) CollectInstance(ctx context.Context, inst model.Instance) model.InspectionResult {
	start := time.Now()
	result := model.InspectionResult{
		InstanceID:    inst.ID,
		Host:          inst.Host,
		Port:          inst.Port,
		CollectedAt:   start,
		SampleSeconds: c.cfg.Collect.SampleIntervalSeconds,
	}
	fail := func(err error) model.InspectionResult {
		result.Error = err.Error()
		result.DurationMS = time.Since(start).Milliseconds()
		return result
	}

	db, err := openMySQL(ctx, inst, c.cfg.Collect)
	if err != nil {
		return fail(err)
	}
	defer db.Close()

	// 第一步：全局变量（后续步骤依赖连接可用性，先做）
	if result.Variables, err = collectVariables(ctx, db); err != nil {
		return fail(err)
	}

	// 第二步：状态采样（两次采样之间有等待间隔），
	// 等待期间并行执行第三步（复制状态）和第四步（自增信息），
	// 单实例总耗时从 "间隔 + 各步骤串行" 缩短为约等于采样间隔。
	before, err := queryGlobalStatus(ctx, db)
	if err != nil {
		return fail(err)
	}

	var (
		rep             *model.ReplicationStatus
		auto            []model.TableAutoIncrement
		errRep, errAuto error
		wg              sync.WaitGroup
	)
	// 复制状态与自增信息互不依赖，各自一个 goroutine，与状态采样等待并行执行
	wg.Add(2)
	go func() {
		defer wg.Done()
		rep, errRep = collectReplication(ctx, db)
	}()
	go func() {
		defer wg.Done()
		auto, errAuto = collectAutoIncrement(ctx, db, c.cfg.Collect.AutoIncrementTopN)
	}()

	select {
	case <-ctx.Done():
		wg.Wait()
		return fail(ctx.Err())
	case <-waitDuration(c.cfg.Collect.SampleIntervalSeconds):
	}
	after, err := queryGlobalStatus(ctx, db)
	wg.Wait()

	// 聚合错误：任何一步失败都记录原因，成功部分仍然保留
	result.StatusDelta = computeDelta(before, after)
	result.StatusRaw = filterKeys(after, statusRawKeys)
	result.Replication = rep
	result.AutoIncrement = auto
	result.Error = joinErrs(err, errRep, errAuto)
	result.DurationMS = time.Since(start).Milliseconds()
	return result
}

// joinErrs 把多个步骤的错误合并为一行描述。
func joinErrs(errs ...error) string {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		if e != nil {
			msgs = append(msgs, e.Error())
		}
	}
	return strings.Join(msgs, "; ")
}
