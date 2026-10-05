package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
)

// TestOutputSink_ChannelOwnership 锁定「stdout 只承载成功数据、错误一律写
// stderr」这条用户可见契约在各出口上的一致归属。
//
// 此前各出口由分散的单点测试分别验证，没有一处把多个出口放在同一断言里
// 对照——某个出口被误接到另一条通道时，缺少跨出口的交叉验证。
//
// 本测试经 newOutputSink 注入缓冲，不猴补进程全局，因此同时验证了
// 注入路径本身可用。
func TestOutputSink_ChannelOwnership(t *testing.T) {
	cases := []struct {
		name     string
		act      func(s *outputSink)
		wantOut  bool
		wantErr  bool
		wantText string
	}{
		{
			name:     "成功信封走成功通道",
			act:      func(s *outputSink) { _ = s.writeOut(envelope.Success("数据")) },
			wantOut:  true,
			wantText: "数据",
		},
		{
			name:     "错误信封走错误通道",
			act:      func(s *outputSink) { _ = s.writeErrJSON(envelope.Error(400, "参数错")) },
			wantErr:  true,
			wantText: "参数错",
		},
		{
			name:     "纯文本行走错误通道",
			act:      func(s *outputSink) { s.writeErrLine("配置告警") },
			wantErr:  true,
			wantText: "配置告警",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			s := newOutputSink(&out, &errBuf)
			c.act(s)

			gotOut, gotErr := out.String(), errBuf.String()
			if c.wantOut && !strings.Contains(gotOut, c.wantText) {
				t.Errorf("期望在成功通道看到 %q，实际成功通道=%q 错误通道=%q", c.wantText, gotOut, gotErr)
			}
			if c.wantErr && !strings.Contains(gotErr, c.wantText) {
				t.Errorf("期望在错误通道看到 %q，实际成功通道=%q 错误通道=%q", c.wantText, gotOut, gotErr)
			}
			// 交叉断言：不该出现的通道必须为空。
			if !c.wantOut && strings.Contains(gotOut, c.wantText) {
				t.Errorf("%q 不应出现在成功通道，实际=%q", c.wantText, gotOut)
			}
			if !c.wantErr && strings.Contains(gotErr, c.wantText) {
				t.Errorf("%q 不应出现在错误通道，实际=%q", c.wantText, gotErr)
			}
		})
	}
}

// TestOutputSink_EnvelopeToSuccessChannelOnly 单独确认成功信封的完整形状：
// 缩进两格、status 字段正确，且错误通道完全不被触碰。
func TestOutputSink_EnvelopeToSuccessChannelOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	s := newOutputSink(&out, &errBuf)
	if err := s.writeOut(envelope.Success(map[string]int{"总数": 3})); err != nil {
		t.Fatalf("writeOut 失败: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("成功信封不应写错误通道，实际写入 %q", errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, `"status": "success"`) {
		t.Errorf("成功信封形状不符: %q", got)
	}
	if !strings.Contains(got, "\n  ") {
		t.Errorf("成功信封应缩进两格: %q", got)
	}
}

// TestOutputSink_WriteFailurePropagates 确认通道不可写时错误向上传递，
// 调用方据此决定退出码——不静默吞掉写入失败。
func TestOutputSink_WriteFailurePropagates(t *testing.T) {
	s := newOutputSink(failWriter{}, &bytes.Buffer{})
	if err := s.writeOut(envelope.Success("x")); err == nil {
		t.Error("成功通道不可写时 writeOut 应返回错误，实际返回 nil")
	}
	s2 := newOutputSink(&bytes.Buffer{}, failWriter{})
	if err := s2.writeErrJSON(envelope.Error(400, "x")); err == nil {
		t.Error("错误通道不可写时 writeErrJSON 应返回错误，实际返回 nil")
	}
}

// failWriter 是恒定失败的 io.Writer，用于模拟通道不可写。
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("通道不可写") }
