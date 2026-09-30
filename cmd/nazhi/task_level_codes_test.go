package main

import (
	"encoding/json"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// runLevelCodes 执行 `nazhi task level-codes` 并返回解析后的 envelope data。
//
// 用既有的 captureStdio（定义在 whoami_test.go）猴补全局通道：
// processOutputSink() 每次写入现取 os.Stdout/os.Stderr，与之配套。
//
// restore 必须显式调用而非 defer：captureStdio 的 drain（io.Copy 到 buffer）
// 就在 restore 闭包内执行，若推迟到本函数返回，函数体内的读取拿到的是空串。
func runLevelCodes(t *testing.T) map[string]map[string]string {
	t.Helper()
	stdout, stderr, restore := captureStdio(t)

	rootCmd.SetArgs([]string{"task", "level-codes"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	execErr := rootCmd.Execute()
	restore()

	if execErr != nil {
		t.Fatalf("nazhi task level-codes 执行失败: %v (stderr=%q)", execErr, stderr.String())
	}

	var env struct {
		Status  envelope.Status              `json:"status"`
		Code    int                          `json:"code"`
		Message string                       `json:"message"`
		Data    map[string]map[string]string `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出应为 JSON envelope: %v (原始=%q)", err, stdout.String())
	}
	if env.Status != envelope.StatusSuccess || env.Code != 200 {
		t.Fatalf("成功信封应为 status=%q code=200，实际 status=%q code=%d",
			envelope.StatusSuccess, env.Status, env.Code)
	}
	return env.Data
}

func TestTaskLevelCodes_ThreeGroupsPresent(t *testing.T) {
	data := runLevelCodes(t)
	for _, key := range []string{"level", "checkResult", "playRole"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("data 缺少 %q 组，实际内容: %v", key, data)
		}
	}
}

func TestTaskLevelCodes_MatchesSDKTables(t *testing.T) {
	data := runLevelCodes(t)
	// 命令输出必须逐项等于 SDK 的表，不允许命令侧另抄一份。
	for key, want := range map[string]map[string]string{
		"level":       types.TaskLevelNames,
		"checkResult": types.CheckResultNames,
		"playRole":    types.PlayRoleNames,
	} {
		got := data[key]
		if len(got) != len(want) {
			t.Fatalf("%s 组条目数不符：命令 %d 项，SDK %d 项", key, len(got), len(want))
		}
		for code, name := range want {
			if got[code] != name {
				t.Errorf("%s[%q]：命令 %q，SDK %q", key, code, got[code], name)
			}
		}
	}
}

func TestTaskLevelCodes_LevelValues(t *testing.T) {
	// 锁定三组的权威取值，防止有人改表时忘了同步前端语义。
	level := runLevelCodes(t)["level"]
	want := map[string]string{
		"1": "国家", "2": "省", "3": "地区/市",
		"4": "区/县/街道/社区", "5": "校", "6": "年段",
	}
	for code, name := range want {
		if level[code] != name {
			t.Errorf("level[%q]=%q，期望 %q", code, level[code], name)
		}
	}
}
