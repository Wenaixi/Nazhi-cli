package logx

import (
	"strings"
	"testing"
)

// TestRedactSnippet_MasksSensitiveValues 锁定诊断摘要的核心契约：
// 敏感键值与 token 查询串必须被掩码。
func TestRedactSnippet_MasksSensitiveValues(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		absent  string
		present string
	}{
		{
			name:    "JSON 键值",
			body:    `{"token":"abc123secret","name":"示例"}`,
			absent:  "abc123secret",
			present: `"token":"***"`,
		},
		{
			name:    "URL 查询串",
			body:    `GET /api/x?token=eyJhbGciOi.J9.zzz&page=1`,
			absent:  "eyJhbGciOi.J9.zzz",
			present: "token=***",
		},
		{
			name:    "学号属 PII",
			body:    `/api/x?userName=G350181200912110035`,
			absent:  "G350181200912110035",
			present: "userName=***",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactSnippet([]byte(c.body))
			if strings.Contains(got, c.absent) {
				t.Errorf("摘要泄漏敏感值 %q: %s", c.absent, got)
			}
			if !strings.Contains(got, c.present) {
				t.Errorf("摘要应含掩码 %q，实际: %s", c.present, got)
			}
		})
	}
}

// TestRedactSnippet_LengthIsCappedByModule 锁定摘要长度由模块单点持有。
//
// 此前该上限是各调用点传入的字面量 100，散落八处且 interface 上不可见；
// 现在调用方只声明用途，长度由 SnippetMaxLen 决定。
func TestRedactSnippet_LengthIsCappedByModule(t *testing.T) {
	long := strings.Repeat("A", 5000)
	got := RedactSnippet([]byte(long))
	if len(got) > SnippetMaxLen {
		t.Errorf("摘要长度 %d 超过模块上限 %d", len(got), SnippetMaxLen)
	}
	if !strings.Contains(got, "A") {
		t.Errorf("摘要应保留前缀内容，实际: %q", got)
	}
}

// TestRedactSnippet_LargeBodySensitiveValueBeyondWindow 锁定粗截窗口的
// 安全性论证：敏感值即使位于响应体深处、落在粗截窗口之外，也不会以
// 「部分保留」的形式泄漏。
//
// 安全性依赖两条：窗口内的敏感值被完整匹配并掩码；窗口外的字节根本不
// 进入输出，因此不存在「值被部分保留而正则失配」的路径。
func TestRedactSnippet_LargeBodySensitiveValueBeyondWindow(t *testing.T) {
	// 敏感值放在粗截窗口（4096 字节）之外。
	padding := strings.Repeat("P", snippetPrefixWindow+500)
	body := padding + `{"token":"LEAKED_SECRET_VALUE"}`
	got := RedactSnippet([]byte(body))

	if strings.Contains(got, "LEAKED_SECRET_VALUE") {
		t.Errorf("窗口外的敏感值不得进入摘要: %s", got)
	}
	// 也不能出现敏感键名后跟部分值的形态（掩码切腰）。
	if strings.Contains(got, `{"token":"LE`) {
		t.Errorf("摘要出现被切断的敏感值前缀: %s", got)
	}
}

// TestRedactSnippet_SensitiveValueInsideWindowIsFullyMasked 锁定窗口内的
// 敏感值被完整掩码——这是粗截窗口存在的意义。
func TestRedactSnippet_SensitiveValueInsideWindowIsFullyMasked(t *testing.T) {
	// 敏感值位于粗截窗口（4096 字节）之内、且在摘要长度（100 字符）之内，
	// 确保它确实进入脱敏流程——这是粗截窗口存在的意义所在。
	padding := strings.Repeat("P", 20)
	body := padding + `{"token":"SECRET_INSIDE_WINDOW"}`
	got := RedactSnippet([]byte(body))
	if strings.Contains(got, "SECRET_INSIDE_WINDOW") {
		t.Errorf("窗口内的敏感值必须被完整掩码: %s", got)
	}
	if !strings.Contains(got, `"token":"***"`) {
		t.Errorf("窗口内的敏感键值应被掩码: %s", got)
	}
}

// TestRedactSnippet_SensitiveValueCrossingTruncationBoundary 锁定
// 「先脱敏后截断」这条安全契约：敏感值跨越截断边界时，先脱敏的路径不泄漏，
// 先截断的路径会因正则失配而泄漏出可辨识前缀。
//
// 这是旧实现（logSafeBody 先裸截再脱敏）的真实故障模式，也是本组测试中
// 唯一能区分两种顺序的用例——其余用例在两种顺序下行为相同，无法证伪。
func TestRedactSnippet_SensitiveValueCrossingTruncationBoundary(t *testing.T) {
	// 夹具要点：敏感键名必须完整落在截断点之前，只有「值」跨越边界。
	// 这样「先截断后脱敏」会留下半个值、正则失配而泄漏；
	// 「先脱敏后截断」则掩码先行，输出不含任何值残片。
	// 若键名本身也跨界，两种顺序都会在键名处截断，行为相同、无法证伪。
	prefix := strings.Repeat("P", SnippetMaxLen-30)
	body := prefix + `{"token":"SECRETVALUE0123456789"}`

	got := RedactSnippet([]byte(body))
	if strings.Contains(got, "SECRET") || strings.Contains(got, "0123456789") {
		t.Errorf("跨越截断边界的敏感值被泄漏: %s", got)
	}
	// 确认不是「整段被截掉」这种假通过：前缀内容应仍在。
	if !strings.Contains(got, "P") {
		t.Errorf("输出应保留前缀内容以证明未被整段截断，实际: %q", got)
	}
	// 掩码应在（脱敏后总长变短，掩码落在截断点之前）。
	if !strings.Contains(got, "***") {
		t.Errorf("输出应含掩码标记，实际: %q", got)
	}
}

// TestRedactSnippet_EmptyAndShortInput 确认短输入与空输入不 panic。
func TestRedactSnippet_EmptyAndShortInput(t *testing.T) {
	for _, in := range []string{"", "短", "{}", "null"} {
		if got := RedactSnippet([]byte(in)); got == "" && in != "" {
			t.Errorf("RedactSnippet(%q) 返回空串", in)
		}
	}
}

// TestRedactSnippetLen_CustomLength 确认按长度变体可用，供确需不同长度的
// 调用方使用。
func TestRedactSnippetLen_CustomLength(t *testing.T) {
	long := strings.Repeat("B", 1000)
	if got := RedactSnippetLen([]byte(long), 20); len(got) > 20 {
		t.Errorf("指定长度 20 时得到 %d 字符", len(got))
	}
}

// TestRedactBody_TruncationDoesNotSplitMask 锁定 RedactBody 的 256 硬上限
// 不会把掩码「切腰」——即敏感值被截成可辨识的残片。
func TestRedactBody_TruncationDoesNotSplitMask(t *testing.T) {
	// 敏感值放在 256 边界附近：脱敏后掩码本身可能跨边界。
	padding := strings.Repeat("P", 240)
	body := padding + `{"token":"SECRET"}`
	got := RedactBody(body)
	if strings.Contains(got, "SECRET") {
		t.Errorf("脱敏后仍含敏感值: %s", got)
	}
}
