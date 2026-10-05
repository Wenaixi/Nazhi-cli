package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/Wenaixi/Nazhi-cli/pkg/types"
)

// timeoutError 是一个模拟超时的 error，实现 Timeout() bool 接口。
// url.Error.Timeout() 和 net.OpError.Timeout() 通过类型断言检查 Err 字段
// 是否实现 Timeout() bool，因此本类型可以让 url.Error/nested OpError 也被
// isTimeoutError 识别为超时错误。
type timeoutError struct{}

func (timeoutError) Error() string { return "i/o timeout" }
func (timeoutError) Timeout() bool { return true }

// ─── ClassifyError 基本分类测试 ───

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorCategory
	}{
		{"context.Canceled", context.Canceled, CategoryContextCancel},
		{"wrapped context.Canceled", fmt.Errorf("wrap: %w", context.Canceled), CategoryContextCancel},
		{"context.DeadlineExceeded", context.DeadlineExceeded, CategoryContextTimeout},
		{"wrapped DeadlineExceeded", fmt.Errorf("wrap: %w", context.DeadlineExceeded), CategoryContextTimeout},
		{"nil error", nil, CategoryUnknown},
		{"other error", errors.New("some error"), CategoryUnknown},
		{"plain ErrBusinessRejected", ErrBusinessRejected, CategoryBusinessError},
		{"wrapped ErrBusinessRejected", fmt.Errorf("op: %w", ErrBusinessRejected), CategoryBusinessError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.want {
				t.Errorf("ClassifyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ─── isTimeoutError 对 context 超时的识别测试 ───

func TestIsTimeoutError_ContextDeadline(t *testing.T) {
	t.Run("context.DeadlineExceeded 被识别为超时", func(t *testing.T) {
		if !isTimeoutError(context.DeadlineExceeded) {
			t.Error("isTimeoutError(context.DeadlineExceeded) = false, want true")
		}
	})
	t.Run("context.Canceled 不被识别为超时（取消≠超时）", func(t *testing.T) {
		if isTimeoutError(context.Canceled) {
			t.Error("isTimeoutError(context.Canceled) = true, want false（取消不是超时）")
		}
	})
	t.Run("context 超时 + url.Error 包装", func(t *testing.T) {
		urlErr := &url.Error{
			Op:  "Get",
			URL: "http://example.com",
			Err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
		}
		if !isTimeoutError(urlErr) {
			t.Error("isTimeoutError(url.Error wrapping DeadlineExceeded) = false, want true")
		}
	})
}

func TestIsTimeoutError_NetworkTimeout(t *testing.T) {
	t.Run("url.Error with timeout", func(t *testing.T) {
		urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: timeoutError{}}
		if !isTimeoutError(urlErr) {
			t.Error("isTimeoutError(url.Error with Timeout) = false, want true")
		}
	})
	t.Run("net.OpError with timeout", func(t *testing.T) {
		netErr := &net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}}
		if !isTimeoutError(netErr) {
			t.Error("isTimeoutError(net.OpError with Timeout) = false, want true")
		}
	})
	t.Run("non-timeout error", func(t *testing.T) {
		if isTimeoutError(errors.New("some error")) {
			t.Error("isTimeoutError(random error) = true, want false")
		}
	})
}

// ─── 网络超时分类测试 ───

func TestClassifyError_NetworkTimeout(t *testing.T) {
	t.Run("url.Error with timeout underlying", func(t *testing.T) {
		urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: timeoutError{}}
		if got := ClassifyError(urlErr); got != CategoryNetworkTimeout {
			t.Errorf("ClassifyError(url.Error with timeout) = %v, want NetworkTimeout", got)
		}
	})

	t.Run("net.OpError with timeout underlying", func(t *testing.T) {
		netErr := &net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}}
		if got := ClassifyError(netErr); got != CategoryNetworkTimeout {
			t.Errorf("ClassifyError(net.OpError with timeout) = %v, want NetworkTimeout", got)
		}
	})

	t.Run("url.Error non-timeout", func(t *testing.T) {
		urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection refused")}
		if got := ClassifyError(urlErr); got == CategoryNetworkTimeout {
			t.Error("non-timeout url.Error should not be NetworkTimeout")
		}
	})
}

// ─── 分类优先级测试 ───

func TestClassifyError_Priority(t *testing.T) {
	t.Run("context.Canceled outranks network timeout", func(t *testing.T) {
		// url.Error wrapping context.Canceled:
		//   errors.Is(urlErr, context.Canceled) 应先于 urlErr.Timeout() 检查
		urlErr := &url.Error{
			Op:  "Get",
			URL: "http://example.com",
			Err: fmt.Errorf("wrapped: %w", context.Canceled),
		}
		if got := ClassifyError(urlErr); got != CategoryContextCancel {
			t.Errorf("期望 ContextCancel，得到 %v", got)
		}
	})

	t.Run("context.DeadlineExceeded outranks network timeout", func(t *testing.T) {
		urlErr := &url.Error{
			Op:  "Get",
			URL: "http://example.com",
			Err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded),
		}
		if got := ClassifyError(urlErr); got != CategoryContextTimeout {
			t.Errorf("期望 ContextTimeout，得到 %v", got)
		}
	})
}

// ─── ParallelDims 错误分类测试 ───

// TestParallelDims_ErrorClassification 锁定 ParallelDims 的错误分桶语义，
// 使「分类 switch 简化」这类纯结构调整可被独立验证。
//
// 分桶口径：context 取消/超时单独成桶（CancelledCount + ContextErrors），
// 其余一律进业务失败桶（FailedCount + BizErrors）。改动前的三分支 switch 中
// NetworkTimeout 与 BusinessError 两分支的循环体逐字相同、default 又与它们
// 相同，故三桶实为两桶。期望值必须是独立字面量，不得写成对 ClassifyError 的
// 二次映射，否则断言与被测实现同源、恒绿。
func TestParallelDims_ErrorClassification(t *testing.T) {
	ctxErr := fmt.Errorf("维度 3(丙): %w", context.Canceled)
	bizErr := fmt.Errorf("维度 5(戊): %w", ErrBusinessRejected)
	otherErr := errors.New("维度 7(庚): 未知故障")

	tests := []struct {
		name          string
		injected      error
		wantCancelled int
		wantFailed    int
		wantCtxBucket bool
		wantBizBucket bool
	}{
		{"context 取消进取消桶", ctxErr, 1, 0, true, false},
		{"业务拒绝进失败桶", bizErr, 0, 1, false, true},
		{"未知错误进失败桶", otherErr, 0, 1, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dims := []types.Dimension{{ID: 1, Name: "甲"}}
			result, _ := ParallelDims(context.Background(), dims, 1, func(context.Context, types.Dimension) ([]types.Task, error) {
				return nil, tt.injected
			})
			if result.CancelledCount != tt.wantCancelled {
				t.Errorf("CancelledCount = %d, want %d", result.CancelledCount, tt.wantCancelled)
			}
			if result.FailedCount != tt.wantFailed {
				t.Errorf("FailedCount = %d, want %d", result.FailedCount, tt.wantFailed)
			}
			if got := len(result.ContextErrors) > 0; got != tt.wantCtxBucket {
				t.Errorf("ContextErrors 非空 = %v, want %v", got, tt.wantCtxBucket)
			}
			if got := len(result.BizErrors) > 0; got != tt.wantBizBucket {
				t.Errorf("BizErrors 非空 = %v, want %v", got, tt.wantBizBucket)
			}
		})
	}
}
