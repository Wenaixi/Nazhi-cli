package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/Nazhi-cli/pkg/types"
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
	// 遍历 task 命令树下**所有**声明了 --level 的子命令，而不是硬编码命令列表：
	// 此前只遍历 submit 与 edit，preview 因同样的 flag 漏掉了表派生而无人发现。
	// 改成从命令树反查后，将来新增同族 flag 命令会自动进入检查范围。
	cmds := taskCommandsWithLevelFlag()
	if len(cmds) == 0 {
		t.Fatal("task 命令树下未找到任何声明 --level 的子命令，测试前提已失效")
	}
	// 至少要有这三个，避免命令树重构后测试静默退化成只检查一个
	wantCmds := map[string]bool{"submit": false, "edit": false, "preview": false}
	for _, cmd := range cmds {
		usage := cmd.Flags().Lookup("level").Usage
		// 码表必须来自 SDK 表：逐项比对，确保没有手写第二份。
		for _, want := range types.TaskLevelNames {
			if !strings.Contains(usage, want) {
				t.Errorf("%s 的 --level usage 缺表内名称 %q：%q", cmd.Name(), want, usage)
			}
		}
		// 旧简写曾长期与表内名称分叉，两者不得同时出现。
		if strings.Contains(usage, "4=区县") {
			t.Errorf("%s 的 --level usage 仍是旧简写：%q", cmd.Name(), usage)
		}
		// 码序是用户可见契约，六个名称齐全但顺序错乱时上两条断言仍会通过。
		// 期望值必须是独立字面量：若写成 taskLevelCodeList()，期望与实际同源于
		// 被测函数，排序一旦出错两边一起错，断言恒成立（变异验证坐实过这一点）。
		const wantCodeList = "1=国家 2=省 3=地区/市 4=区/县/街道/社区 5=校 6=年段"
		if !strings.Contains(usage, wantCodeList) {
			t.Errorf("%s 的 --level usage 未按 1..6 顺序给出码表：%q", cmd.Name(), usage)
		}
		if _, ok := wantCmds[cmd.Name()]; ok {
			wantCmds[cmd.Name()] = true
		}
	}
	for name, found := range wantCmds {
		if !found {
			t.Errorf("命令 %s 应声明 --level 但未被本测试遍历到", name)
		}
	}
}

// taskCommandsWithLevelFlag 返回 task 命令树下所有声明了 --level 的子命令。
func taskCommandsWithLevelFlag() []*cobra.Command {
	var out []*cobra.Command
	for _, c := range taskCmd.Commands() {
		if c.Flags().Lookup("level") != nil {
			out = append(out, c)
		}
	}
	return out
}
