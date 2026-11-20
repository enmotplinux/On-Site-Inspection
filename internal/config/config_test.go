package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaults(t *testing.T) {
	path := writeTemp(t, `{"mongodb": {"host": "10.0.0.1", "port": 27018}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// JSON 覆盖的字段
	if cfg.MongoDB.Host != "10.0.0.1" || cfg.MongoDB.Port != 27018 {
		t.Errorf("JSON 覆盖失败: %+v", cfg.MongoDB)
	}
	// 未给的字段使用默认值
	if cfg.MongoDB.Database != "onsite_inspection" {
		t.Errorf("默认库名错误: %s", cfg.MongoDB.Database)
	}
	if cfg.Collect.Concurrency != 5 || cfg.Collect.SampleIntervalSeconds != 1 {
		t.Errorf("默认采集参数错误: %+v", cfg.Collect)
	}
	if cfg.Collect.SampleInterval() != 1_000_000_000 {
		t.Errorf("SampleInterval 换算错误: %v", cfg.Collect.SampleInterval())
	}
}

func TestLoadEnvPasswordOverride(t *testing.T) {
	t.Setenv("ONSITE_MONGO_PASSWORD", "secret-from-env")
	path := writeTemp(t, `{"mongodb": {"host": "127.0.0.1", "port": 27017, "password": "plain"}}`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MongoDB.Password != "secret-from-env" {
		t.Errorf("环境变量应覆盖文件密码, got %q", cfg.MongoDB.Password)
	}
}

func TestLoadValidate(t *testing.T) {
	// 显式空 host（未写 host 字段时会回落到默认值，属预期行为）
	path := writeTemp(t, `{"mongodb": {"host": "", "port": 27017}}`)
	if _, err := Load(path); err == nil {
		t.Error("空 host 应报错")
	}
	// 并发数非法
	path = writeTemp(t, `{"mongodb": {"host": "127.0.0.1"}, "collect": {"concurrency": 0}}`)
	if _, err := Load(path); err == nil {
		t.Error("concurrency=0 应报错")
	}
}

func TestURI(t *testing.T) {
	tests := []struct {
		name string
		cfg  MongoDBConfig
		want string
	}{
		{
			name: "带认证",
			cfg:  MongoDBConfig{Host: "127.0.0.1", Port: 27017, User: "tplinux", Password: "red@hat", AuthDatabase: "abc"},
			want: "mongodb://tplinux:red%40hat@127.0.0.1:27017/abc",
		},
		{
			name: "无认证",
			cfg:  MongoDBConfig{Host: "127.0.0.1", Port: 27017},
			want: "mongodb://127.0.0.1:27017",
		},
		{
			name: "用户无密码",
			cfg:  MongoDBConfig{Host: "h", Port: 27017, User: "u", AuthDatabase: "admin"},
			want: "mongodb://u@h:27017/admin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.URI(); got != tt.want {
				t.Errorf("URI() = %q, want %q", got, tt.want)
			}
		})
	}
}
