package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
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
	// 锁定命令输出是 SDK 表的逐项透传，没有按命令侧的另一份文案加工或裁剪。
	// 注意本测试与命令读的是同一张表，因此抓不到「命令侧另抄一份内容相同的
	// 字面量」——那类漂移由 TestTaskLevelCodes_GroupValues 的独立字面量期望值守住。
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

func TestTaskLevelCodes_GroupValues(t *testing.T) {
	// 锁定三组的权威取值，防止有人改表时忘了同步前端语义。
	// want 必须是字面量：一旦改成引用 SDK 表，命令与期望就同源，
	// 断言退化成立即比较，也锁不住「命令侧另抄一份内容相同的字面量」。
	groups := []struct {
		key  string
		want map[string]string
	}{
		{"level", map[string]string{
			"1": "国家", "2": "省", "3": "地区/市",
			"4": "区/县/街道/社区", "5": "校", "6": "年段",
		}},
		{"checkResult", map[string]string{
			"1": "优秀", "2": "良", "3": "合格", "4": "差",
		}},
		{"playRole", map[string]string{
			"1": "主持策划者", "2": "主要参与者", "3": "参与者",
		}},
	}
	data := runLevelCodes(t)
	for _, g := range groups {
		t.Run(g.key, func(t *testing.T) {
			got := data[g.key]
			// 条目数一并比对：只逐项查字面量的话，表里多出一项无人认领的
			// 新码（如 "7"）也会全绿。
			if len(got) != len(g.want) {
				t.Fatalf("%s 组条目数不符：命令 %d 项，期望 %d 项", g.key, len(got), len(g.want))
			}
			for code, name := range g.want {
				if got[code] != name {
					t.Errorf("%s[%q]=%q，期望 %q", g.key, code, got[code], name)
				}
			}
		})
	}
}

func TestTaskLevelUsage_DerivedFromTable(t *testing.T) {
	// --level 的 usage 文案是用户可见契约，必须与 SDK 表一致。
	// 「4=区县」与表里的「区/县/街道/社区」曾长期分叉，此测试锁住收敛结果。
	for _, cmd := range []*cobra.Command{taskSubmitCmd, taskEditCmd} {
		usage := cmd.Flags().Lookup("level").Usage
		if !strings.Contains(usage, "4=区/县/街道/社区") {
			t.Errorf("%s 的 --level usage 未使用表内名称：%q", cmd.Name(), usage)
		}
		if strings.Contains(usage, "4=区县") {
			t.Errorf("%s 的 --level usage 仍是旧简写：%q", cmd.Name(), usage)
		}
		// 码序同样是用户可见契约：即使六个名称都在，输出成「4=… 1=国家」这种
		// 乱序仍会让用户对不上号，所以整句比对而不只比对片段。
		want := "等级代码（可选，写实：1=国家 2=省 3=地区/市 4=区/县/街道/社区 5=校 6=年段；空则原样不默认 5）"
		if usage != want {
			t.Errorf("%s 的 --level usage 与期望整句不一致：\n实际 %q\n期望 %q", cmd.Name(), usage, want)
		}
	}
}
