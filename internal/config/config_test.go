package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"butler/internal/parser"
)

func TestSavePlan_原子写入并能重新加载状态(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	firedAt := time.Date(
		2026,
		8,
		26,
		15,
		30,
		0,
		0,
		time.FixedZone("CST", 8*60*60),
	)
	plan := &parser.PlanNode{
		Children: []parser.PlanNode{
			{
				Title:     "已执行任务",
				LastFired: firedAt,
			},
			{
				Title: "未执行任务",
			},
		},
	}

	if err := SavePlan(plan); err != nil {
		t.Fatalf("SavePlan() 返回错误：%v", err)
	}

	loaded, err := LoadPlan()
	if err != nil {
		t.Fatalf("LoadPlan() 返回错误：%v", err)
	}
	if len(loaded.Children) != 2 {
		t.Fatalf("重新加载后的节点数量 = %d，期望 2", len(loaded.Children))
	}
	if !loaded.Children[0].LastFired.Equal(firedAt) {
		t.Fatalf(
			"重新加载的 LastFired = %s，期望 %s",
			loaded.Children[0].LastFired,
			firedAt,
		)
	}
	if !loaded.Children[1].LastFired.IsZero() {
		t.Fatalf("未执行任务的 LastFired = %s，期望零值", loaded.Children[1].LastFired)
	}

	planPath := filepath.Join(configRoot, "butler", "plan-new.jsonc")
	raw, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("读取保存后的计划文件：%v", err)
	}
	if got := bytes.Count(raw, []byte(`"LastFired"`)); got != 1 {
		t.Fatalf("计划文件中的 LastFired 字段数量 = %d，期望 1", got)
	}

	var document struct {
		Children []map[string]json.RawMessage
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("解析保存后的计划文件：%v", err)
	}
	if len(document.Children) != 2 {
		t.Fatalf("保存后的节点数量 = %d，期望 2", len(document.Children))
	}

	assertJSONKeys(
		t,
		document.Children[0],
		[]string{"Title", "LastFired"},
	)
	assertJSONKeys(
		t,
		document.Children[1],
		[]string{"Title"},
	)

	var lastFiredText string
	if err := json.Unmarshal(
		document.Children[0]["LastFired"],
		&lastFiredText,
	); err != nil {
		t.Fatalf("解析 LastFired 字符串：%v", err)
	}
	if want := firedAt.Format(time.RFC3339Nano); lastFiredText != want {
		t.Fatalf("LastFired 字符串 = %q，期望 %q", lastFiredText, want)
	}

	tempPath := planPath + ".tmp"
	if _, err := os.Stat(tempPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("临时文件 %s 在成功替换后仍然存在，stat error = %v", tempPath, err)
	}
}

func assertJSONKeys(
	t *testing.T,
	object map[string]json.RawMessage,
	want []string,
) {
	t.Helper()

	if len(object) != len(want) {
		t.Fatalf("JSON 字段 = %v，期望 %v", mapKeys(object), want)
	}
	for _, key := range want {
		if _, ok := object[key]; !ok {
			t.Fatalf("JSON 字段 = %v，缺少 %q", mapKeys(object), key)
		}
	}
}

func mapKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}
