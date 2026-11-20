# On-Site-Inspection（Go 版）

MySQL 现场巡检工具：并发采集实例清单、全局参数、状态差值、复制状态、业务表自增使用情况，结果存入 MongoDB，并生成带风险检查的终端报告。

原 Python 版本说明见文末[历史章节](#原-python-版说明历史资料原样保留)。

## 构建

```bash
go build -o bin/onsite ./cmd/onsite
```

要求 Go 1.23+，无其他环境依赖。

## 配置

配置文件 `conf/config.json`（查找顺序：`ONSITE_CONFIG` 环境变量 → `./conf/config.json` → 可执行文件同级/上级 `conf/`）：

```json
{
    "mongodb": {
        "host": "127.0.0.1",
        "port": 27017,
        "user": "",
        "password": "",
        "database": "onsite_inspection",
        "auth_database": "admin"
    },
    "collect": {
        "concurrency": 5,
        "sample_interval_seconds": 1,
        "connect_timeout_seconds": 5,
        "read_timeout_seconds": 10,
        "result_retention": 20,
        "auto_increment_top_n": 10
    }
}
```

支持的环境变量：

| 变量 | 用途 |
| ---- | ---- |
| `ONSITE_CONFIG` | 指定配置文件路径 |
| `ONSITE_MONGO_PASSWORD` | 覆盖配置文件中的 MongoDB 密码（避免明文落盘） |
| `ONSITE_MYSQL_PASSWORD` | `add` 录入实例时的 MySQL 密码（避免终端交互） |
| `NO_COLOR` | 禁用报告的 ANSI 颜色输出 |

## 快速开始

```bash
# 1. 录入待巡检实例（密码支持 -password 参数 / 环境变量 / 终端隐藏输入）
./bin/onsite add -host 192.168.1.10 -port 3306 -user root

# 2. 查看已录入实例
./bin/onsite list

# 3. 并发巡检全部实例（实时逐台输出 OK/FAIL，结果存入 MongoDB，每实例保留最近 20 份）
./bin/onsite run

# 4. 查看报告（省略 -id 输出全部实例）
./bin/onsite report -id 192.168.1.10_3306

# 5. 删除实例
./bin/onsite remove -id 192.168.1.10_3306
```

报告顶部为自动风险检查：双一参数、复制线程/延迟、连接使用率（>=80%）、自增值临近 INT 溢出等，`[RISK]`/`[WARN]` 分级；主体按"关键参数 / 流量与操作 / 等待与并发 / 连接与文件 / 复制状态 / 自增使用"分组展示，字节参数自动附 K/M/G 单位。

## 架构

```
onsite-inspection/
├── cmd/onsite/main.go          # CLI 入口：add / list / remove / run / report
├── conf/config.json            # 配置
└── internal/
    ├── config/                 # 配置加载：默认值 + 环境变量覆盖 + 多路径查找
    ├── model/                  # 数据模型：Instance / InspectionResult / Summary
    ├── storage/                # Store 接口 + MongoDB 实现（连接池复用、结果保留、复合索引）
    ├── collector/              # MySQL 采集（变量/状态差值/复制/自增）+ 两级并发调度
    └── report/                # 报告渲染：风险检查 + ANSI 配色 + tabwriter 对齐
```

并发模型：多实例之间由信号量控制的 goroutine 并发（默认 5）；实例内部，状态采样的等待间隔里并行采集复制状态与自增信息，单实例耗时约等于采样间隔。

兼容 MySQL 5.7 / 8.0 / 8.4+（`SHOW SLAVE STATUS` 语法错误时自动回退 `SHOW REPLICA STATUS`），状态差值自动处理计数器回绕（实例重启 / FLUSH STATUS）。

## 与原 Python 版的主要差异

| 原版问题 | Go 版处理 |
| -------- | --------- |
| 每次操作重建 MongoDB 连接（一次巡检连 4 次） | 连接池全程复用 |
| `eval()` 解析存储的 JSON 字符串 | 结构化 BSON 文档直接解码 |
| 相对路径 `./conf/conf`，依赖运行目录 | 多路径查找配置，任意目录可运行 |
| 单实例失败即崩溃（无异常处理） | 错误隔离：失败落库带 error 的结果，继续其他实例 |
| 串行巡检 | 两级并发 + 实时进度输出 |
| 明文打印密码 | 密码支持环境变量 / 终端隐藏输入，错误信息脱敏 |
| 状态差值可能为负（计数器回绕） | 新值小于旧值时取新值 |
| 采集了复制/自增数据但不展示 | 报告完整展示并做风险检查 |
| 结果无限增长 | 每实例保留最近 N 份（默认 20） |

---

## 原 Python 版说明（历史资料，原样保留）

-1.要先进行mongo安装和配置 启动用户和密码 创建abc库

-mongo 用户配置在conf下面conf
-2.启动要巡检的MySQL程序
-收集要巡检的MySQL的信息
-执行
-bin目录下面inser_mysql.py
-录入相关信息
-3.录入好之后 执行 index.py
-把巡检信息收集到mongo中
-4.通过 bin目录下select_mysql 进行巡检分析
-
-5. lib下面二个文件 是MySQL的一个收集参数文件和手机状态信息文件
-
-index.py 是收集程序
-
-内部使用 没做异常处理
