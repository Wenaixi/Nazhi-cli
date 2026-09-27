package logx

import (
	"strings"
	"testing"
)

// TestRedactSnippet_SensitiveValueCrossingPrefixWindow 锁定粗截窗口边界的
// 另一侧：当敏感「值」本身长到跨过 snippetPrefixWindow，窗口把它切断后
// kvRe 因缺少闭合引号而失配，掩码不会生效。
//
// 与 TestRedactSnippet_LargeBodySensitiveValueBeyondWindow 的区别：那条
// 把敏感值整个放在窗口之外，字节根本不进入输出，因而对任何实现都必然通过；
// 本条把敏感「键名」放在窗口内、「值」跨越边界——这是唯一能区分「掩码生效」
// 与「掩码失配」的形态。
func TestRedactSnippet_SensitiveValueCrossingPrefixWindow(t *testing.T) {
	long := strings.Repeat("A", snippetPrefixWindow)
	body := []byte(`{"token":"` + long + `","msg":"end"}`)

	got := RedactSnippet(body)

	// 键名在窗口内、值被切断时，摘要不得出现任何明文值残片。
	if !strings.Contains(got, `"token":"***"`) {
		t.Errorf("跨粗截窗口的敏感值必须掩码，实际: %q", got)
	}
	// 明文残片（连续 A）一旦出现即说明掩码未生效。
	if strings.Contains(got, "AAAA") {
		t.Errorf("摘要泄漏敏感值明文前缀: %q", got)
	}
}

// TestRedactSnippet_TruncatedSensitiveValueAtWindowEdge 锁定窗口边界恰好
// 切在值中间那一格：键名完整、值恰好被切掉一部分。
func TestRedactSnippet_TruncatedSensitiveValueAtWindowEdge(t *testing.T) {
	// 使 body 长度超过窗口，且敏感值起点在窗口内。
	prefix := strings.Repeat("P", snippetPrefixWindow-20)
	body := []byte(prefix + `{"token":"` + strings.Repeat("S", 500) + `"}`)

	got := RedactSnippet(body)

	if strings.Contains(got, "SSSS") {
		t.Errorf("窗口边界切断的敏感值未掩码: %q", got)
	}
}
