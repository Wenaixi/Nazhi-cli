// file_upload_id_normalize_test.go 锁定 UploadFile 对 returnData.id 的归一口径。
//
// 该值是平台返回的附件标识，经 types.NormalizeInteger 单点归一。
// 本文件锁定的三条边界都命中生产路径：用例的 id 全部以 JSON 整数字面量
// 出现在响应中，file.go 取 id 键的原始字节直接送归一（不从已解码的 map 取，
// 因归一模块的范围判定针对原始 JSON 字面量定义）。
//
// 越界与非整值必须显式报错而非静默产出错误值：静默回绕会让
// task.go 的 pictureList 收到负数附件 ID，错误推迟到服务端才暴露，
// 排查方向被误导到远端。
package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// newUploadIDServer 起一个只回固定 returnData 的上传服务。
func newUploadIDServer(t *testing.T, returnData string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"code":1,"returnData":%s}`, returnData)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeUploadProbeFile 造一个走直传的纯文本文件（不经图片预处理）。
func writeUploadProbeFile(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/probe.txt"
	if err := os.WriteFile(path, []byte("probe"), 0o600); err != nil {
		t.Fatalf("写临时文件失败: %v", err)
	}
	return path
}

// TestUploadFile_RejectsOutOfRangeAttachmentID 锁定超出 int64 范围的 id 必须报错。
//
// 回归场景：int64(f) 对 2^63 直接转换会回绕为 math.MinInt64，
// 产出「正 ID 变负数」的错误值且不报错。
func TestUploadFile_RejectsOutOfRangeAttachmentID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		returnData string
	}{
		{"int64 上界之外", `{"id":9223372036854775808}`},
		{"远超 int64", `{"id":10000000000000000000}`},
		{"科学计数大数", `{"id":1e30}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newUploadIDServer(t, tc.returnData)
			c, err := New(WithUploadURL(srv.URL), WithTimeout(5*time.Second))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			defer c.Close()

			got, err := c.UploadFile(t.Context(), writeUploadProbeFile(t))
			if err == nil {
				t.Fatalf("越界附件 ID 必须报错，实际返回 AttachmentID=%d", got.AttachmentID)
			}
			if !errors.Is(err, ErrUploadRejected) {
				t.Errorf("越界 id 应归 ErrUploadRejected，实际 %v", err)
			}
		})
	}
}

// TestUploadFile_RejectsNonIntegerAttachmentID 锁定非整值 id 必须报错。
//
// 回归场景：int64(3.9) 静默截断为 3，不报错。
func TestUploadFile_RejectsNonIntegerAttachmentID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		returnData string
	}{
		{"非整值浮点", `{"id":3.9}`},
		{"负小数", `{"id":-0.5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newUploadIDServer(t, tc.returnData)
			c, err := New(WithUploadURL(srv.URL), WithTimeout(5*time.Second))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			defer c.Close()

			if _, err := c.UploadFile(t.Context(), writeUploadProbeFile(t)); err == nil {
				t.Fatal("非整值附件 ID 必须报错")
			}
		})
	}
}

// TestUploadFile_NonPositiveAttachmentIDRejected 锁定归零与负数 id 不得放行。
//
// 归一模块把缺省形态（缺键/null/空串）归零是刻意的，但「归零不等于放行」：
// 附件 ID 为零或负数在业务上无意义，会流入 pictureList 变成无效载荷。
func TestUploadFile_NonPositiveAttachmentIDRejected(t *testing.T) {
	for _, tc := range []struct {
		name       string
		returnData string
	}{
		{"零值", `{"id":0}`},
		{"负数", `{"id":-7}`},
		{"缺 id 键", `{"name":"a.txt"}`},
		{"null", `{"id":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newUploadIDServer(t, tc.returnData)
			c, err := New(WithUploadURL(srv.URL), WithTimeout(5*time.Second))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			defer c.Close()

			if _, err := c.UploadFile(t.Context(), writeUploadProbeFile(t)); err == nil {
				t.Fatal("非正数附件 ID 必须报错")
			}
		})
	}
}

// TestUploadFile_AcceptsRealisticAttachmentID 确保修复不过度收紧：
// 正常形态（整值浮点、数字字符串）仍须解出正确值。
func TestUploadFile_AcceptsRealisticAttachmentID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		returnData string
		want       int64
	}{
		{"裸整数", `{"id":5139876}`, 5139876},
		{"整值浮点", `{"id":5139876.0}`, 5139876},
		{"数字字符串", `{"id":"5139876"}`, 5139876},
		{"int64 上界", `{"id":9223372036854775807}`, 9223372036854775807},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := newUploadIDServer(t, tc.returnData)
			c, err := New(WithUploadURL(srv.URL), WithTimeout(5*time.Second))
			if err != nil {
				t.Fatalf("建客户端失败: %v", err)
			}
			defer c.Close()

			got, err := c.UploadFile(t.Context(), writeUploadProbeFile(t))
			if err != nil {
				t.Fatalf("合法附件 ID 不应报错: %v", err)
			}
			if got.AttachmentID != tc.want {
				t.Errorf("AttachmentID = %d, 期望 %d", got.AttachmentID, tc.want)
			}
		})
	}
}
