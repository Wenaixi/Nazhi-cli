package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestMain_ParamErrorExitsThree 锁定参数错误的真实生产出口：写 stderr、退出码 3。
//
// 此前此处有个 TestMain_NoDoubleErrorOutput，它自己复制了生产那一行
// printEnvelope(envelope.Error(400, ...)) 并断言 stdout 含 error、stderr 为空。
// 生产早已改走 printParamError（写 stderr），而该测试仍断言旧世界——它测的是
// 自己复制的那一行，生产改了它不会红。变异验证可复现：把那行换成生产的
// printParamError，测试立即报「stderr 应为空，实际含 error 信封」。两条相反契约
// 并存（param_error_channel_test.go 断言 stdout 必须为空），属恒绿且失实的测试，
// 已删除。
//
// 本用例直接验证 printParamError 本身——main.go 收到 cobra 参数解析错误后调它，
// 退出码由它设置。不复制 main.go 的控制流：复制生产逻辑正是原测试恒绿的根因。
// 通道归属另有 param_error_channel_test.go 与 TestNoDirectStdoutWritesInCmd 守卫。
func TestMain_ParamErrorExitsThree(t *testing.T) {
	// pendingExitCode 是进程级状态，用例结束后复位以免污染其它用例。
	defer pendingExitCode.Store(0)

	origStdout, origStderr := os.Stdout, os.Stderr
	defer func() { os.Stdout, os.Stderr = origStdout, origStderr }()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stderr 失败: %v", err)
	}
	os.Stdout, os.Stderr = stdoutW, stderrW

	printParamError(errors.New("unknown flag: --badflag"))

	// 关 writer 让 reader 能读到 EOF
	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	if _, err := io.Copy(&stdoutBuf, stdoutR); err != nil {
		t.Fatalf("读取 stdout 失败: %v", err)
	}
	if _, err := io.Copy(&stderrBuf, stderrR); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}

	if code := pendingExitCode.Load(); code != 3 {
		t.Errorf("参数错误应标记退出码 3，实际 %d", code)
	}
	if got := stdoutBuf.String(); got != "" {
		t.Errorf("stdout 应为空（错误一律写 stderr），实际: %q", got)
	}
	if got := stderrBuf.String(); !strings.Contains(got, `"code": 400`) {
		t.Errorf("stderr 应含 400 参数错误信封，实际: %q", got)
	}
}

// TestRootCmd_HasSilenceFlags 直接断言 package-level rootCmd 已经设置了
// SilenceErrors 和 SilenceUsage（main.go init() 阶段生效）。
// 防止 init() 里忘了加导致回归。
func TestRootCmd_HasSilenceFlags(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors 应为 true（防止 cobra 自带重复错误输出）")
	}
	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage 应为 true（防止 cobra 自带重复 usage 输出）")
	}
}

// 引入 cobra 引用以让编译器保留 cobra 包导入（虽然 cobra 已被 rootCmd 间接引用）
var _ = cobra.NoArgs
