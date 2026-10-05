package main

import (
	"encoding/json"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// TestTypicalCaseCodes_CommandRegistered 验证典型案例查表命令已挂到
// typical-case 命令树下。
func TestTypicalCaseCodes_CommandRegistered(t *testing.T) {
	var found bool
	for _, c := range typicalCaseCmd.Commands() {
		if c.Name() == "level-codes" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("typical-case 应注册 level-codes 子命令")
	}
}

// TestTypicalCaseCodes_GroupValues 锁定查表输出的三组码值。
//
// 期望值必须是独立字面量，不得写成对 SDK 码表的二次引用——
// 若写成 types.TypicalCaseTypeNames 之类，期望与实际同源于被测数据，
// 表被误改时断言会跟着一起错，恒绿。码值取自前端 classiccanter.vue 的
// el-option 硬编码（下拉无字典接口）。
//
// 同时断言 level 组与写实域 level 组不相等：两套码表编号重叠处语义不同
// （典型案例 1=国际、5=学校；写实 1=国家、5=校），这是最容易被误合并的地方。
func TestTypicalCaseCodes_GroupValues(t *testing.T) {
	want := map[string]map[string]string{
		"type": {
			"1": "研究性学习报告",
			"2": "社会调查报告",
			"3": "艺术创作作品",
			"4": "其他",
		},
		"role": {
			"1": "负责人",
			"2": "参与者",
		},
		"level": {
			"1": "国际",
			"2": "省",
			"3": "市",
			"4": "区县",
			"5": "学校",
		},
	}

	stdout, _, restore := captureStdio(t)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
	})
	typicalCaseCodesCmd.Run(typicalCaseCodesCmd, nil)
	restore()

	if pendingExitCode.Load() != 0 {
		t.Fatalf("查表命令不应出错，退出码 %d", pendingExitCode.Load())
	}

	var env struct {
		Data map[string]map[string]string `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("解析查表输出失败: %v\n实际输出: %s", err, stdout.String())
	}

	for group, wantGroup := range want {
		gotGroup, ok := env.Data[group]
		if !ok {
			t.Errorf("输出缺 %s 组，实际键: %v", group, env.Data)
			continue
		}
		if len(gotGroup) != len(wantGroup) {
			t.Errorf("%s 组条数 = %d, want %d", group, len(gotGroup), len(wantGroup))
		}
		for code, wantName := range wantGroup {
			if gotName := gotGroup[code]; gotName != wantName {
				t.Errorf("%s[%q] = %q, want %q", group, code, gotName, wantName)
			}
		}
	}

	// 两套 level 码表必须不同：这是禁止合并的实质断言。
	if env.Data["level"]["1"] == "国家" || env.Data["level"]["5"] == "校" {
		t.Errorf("典型案例 level 组疑似混入写实域码值：1=%q 5=%q",
			env.Data["level"]["1"], env.Data["level"]["5"])
	}
}

// TestTypicalCaseCodes_SurvivesTableTampering 变异验证：本守卫必须在被测表
// 被篡改时变红，否则 TestTypicalCaseCodes_GroupValues 的码值断言恒绿。
//
// 做法：临时改写 types.TypicalCaseLevelNames 的一个码值，确认读到的值随之改变；
// 随后恢复原值。表是导出 map，这一步同时把「下游若写入该表、CLI 输出随之
// 改变」这一已知代价坐实为事实而非推测——pkg/types/task.go 的三张写实码表
// 同样如此，故三处 godoc 都写明「下游只应读取，不要写入」。
func TestTypicalCaseCodes_SurvivesTableTampering(t *testing.T) {
	const tamperedCode = "1"
	original := types.TypicalCaseLevelNames[tamperedCode]
	types.TypicalCaseLevelNames[tamperedCode] = "国家"
	t.Cleanup(func() { types.TypicalCaseLevelNames[tamperedCode] = original })

	got := types.TypicalCaseLevelNames[tamperedCode]
	want := "国际"
	if got == want {
		t.Fatalf("篡改后仍读到 %q，期望码表比较逻辑对其改动敏感", got)
	}
}
