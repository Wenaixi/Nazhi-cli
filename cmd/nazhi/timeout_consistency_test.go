package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTimeoutTestCmd 构造带 timeout flag 的最小命令。
func newTimeoutTestCmd(flagValue string, flagSet bool) *cobra.Command {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Int("timeout", 15, "")
	if flagSet {
		_ = cmd.Flags().Set("timeout", flagValue)
	}
	return cmd
}

// TestResolveTimeoutSec_SameOutcomeForFlagAndEnv --timeout 0 与 NAZHI_TIMEOUT=0
// 是同一类非法输入，必须同果：回退注册默认 15 且都告警。此前 flag 非法告警后
// 回退 30、env 非法静默回退 30、未设置却是 15，三通道不一致。
func TestResolveTimeoutSec_SameOutcomeForFlagAndEnv(t *testing.T) {
	t.Setenv("NAZHI_TIMEOUT", "0")
	_, stderr, restore := captureStdio(t)
	got := resolveTimeoutSec(newTimeoutTestCmd("0", true), "NAZHI_TIMEOUT")
	restore()
	if got != 15 {
		t.Errorf("flag 非法应回退 15，实际 %d", got)
	}
	if !strings.Contains(stderr.String(), "warn") {
		t.Errorf("flag 非法应有告警，实际 stderr=%q", stderr.String())
	}

	_, stderr2, restore2 := captureStdio(t)
	gotEnv := resolveTimeoutSec(newTimeoutTestCmd("", false), "NAZHI_TIMEOUT")
	restore2()
	if gotEnv != 15 {
		t.Errorf("env 非法应回退 15，实际 %d", gotEnv)
	}
	if !strings.Contains(stderr2.String(), "warn") {
		t.Errorf("env 非法应有告警，此前是静默回退。实际 stderr=%q", stderr2.String())
	}
}

// TestResolveTimeoutSec_ValidPathsUnchanged 合法路径不受收敛影响：
// flag 显式值优先于 env；未设置时用注册默认 15；正数 env 生效。
func TestResolveTimeoutSec_ValidPathsUnchanged(t *testing.T) {
	t.Setenv("NAZHI_TIMEOUT", "")
	if got := resolveTimeoutSec(newTimeoutTestCmd("30", true), "NAZHI_TIMEOUT"); got != 30 {
		t.Errorf("显式 flag 应生效，实际 %d", got)
	}
	if got := resolveTimeoutSec(newTimeoutTestCmd("", false), "NAZHI_TIMEOUT"); got != 15 {
		t.Errorf("未设置应用注册默认 15，实际 %d", got)
	}

	t.Setenv("NAZHI_TIMEOUT", "45")
	if got := resolveTimeoutSec(newTimeoutTestCmd("", false), "NAZHI_TIMEOUT"); got != 45 {
		t.Errorf("合法 env 应生效，实际 %d", got)
	}
}

// newTimeoutTestCmdWithDefault 构造注册默认值为 def 的 timeout 命令。
func newTimeoutTestCmdWithDefault(def int) *cobra.Command {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Int("timeout", def, "")
	return cmd
}

// TestResolveTimeoutSec_WarningReportsOwnRegisteredDefault 锁定告警文案不得
// 谎报回退值：文案里的「默认 N 秒」必须是**该命令自身的 flag 注册默认值**，
// 而不是函数内硬编码的常量。
//
// file upload / file download 注册默认值是 30（见其 Flags().Int 调用），
// 业务命令是 15。若告警固定说 15，file 命令的用户会在 --help 里读到
// (default 30) 却被告警告知改成 15——同一 flag 两套承诺。
//
// 该用例与既有 newTimeoutTestCmd（注册默认 15）构成对照：修复前两者文案相同，
// 本用例恒绿。
func TestResolveTimeoutSec_WarningReportsOwnRegisteredDefault(t *testing.T) {
	t.Setenv("NAZHI_TIMEOUT", "")

	for _, def := range []int{15, 30} {
		cmd := newTimeoutTestCmdWithDefault(def)
		_ = cmd.Flags().Set("timeout", "0")

		_, stderr, restore := captureStdio(t)
		got := resolveTimeoutSec(cmd, "NAZHI_TIMEOUT")
		restore()

		if got != def {
			t.Errorf("注册默认 %d 的命令在 flag 非法时应回退 %d，实际 %d", def, def, got)
		}
		want := fmt.Sprintf("使用默认 %d 秒超时", def)
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("告警应报告该命令自身的注册默认 %d（文案含 %q），实际 stderr=%q",
				def, want, stderr.String())
		}
	}
}
