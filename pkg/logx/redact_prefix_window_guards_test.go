package logx

import (
	"strings"
	"testing"
)

// 跨粗截窗口的守卫测试。
//
// 夹具的共同要点（弄错就会变成对任何实现都通过的恒绿测试）：
//
//  1. 敏感键名必须落在摘要上限 SnippetMaxLen 之内。粗截窗口把值切腰后，
//     掩码结果随脱敏流到 RedactBodyThenTruncate，最后被截到 SnippetMaxLen
//     字符。若键名本身就在 100 字符之外，整段连同掩码一起被截掉，断言里的
//     明文永不可能出现——测试不报错也不代表安全。
//  2. 敏感值必须长到跨过 snippetPrefixWindow，且 body 总长超过该窗口，
//     否则 clipPrefixWindow 走原样返回分支，残段替换根本没发生。
//
// 因此统一把敏感键放在 JSON 开头，值取 snippetPrefixWindow 长：键名必然在
// 摘要长度之内，值必然跨界。

// TestRedactSnippet_CrossingWindow_AllSensitiveKeys 锁定六个敏感键各自的
// 跨窗形态都被掩码。
//
// 只用 token 一种键做夹具时，把其余键名在 kvUnterminatedTailRe 里拼错或
// 漏掉（passwd 与 password 只差一个字母、x-auth-token 含连字符最易出错）
// 不会有任何测试失败——那类缺陷恰好只在真实响应里暴露。
func TestRedactSnippet_CrossingWindow_AllSensitiveKeys(t *testing.T) {
	for _, key := range []string{"token", "x-auth-token", "authorization", "password", "passwd", "captcha"} {
		t.Run(key, func(t *testing.T) {
			body := []byte(`{"` + key + `":"` + strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`)

			got := RedactSnippet(body)

			if !strings.Contains(got, `"`+key+`":"***"`) {
				t.Errorf("敏感键 %q 跨粗截窗口时未被掩码，实际: %q", key, got)
			}
			if strings.Contains(got, "AAAA") {
				t.Errorf("敏感键 %q 跨粗截窗口时泄漏明文值，实际: %q", key, got)
			}
		})
	}
}

// TestRedactSnippet_CrossingWindow_CaseInsensitive 锁定跨窗匹配大小写不敏感。
//
// kvUnterminatedTailRe 带 (?i) 才有此性质；去掉它，大写键名的残段就与该
// 正则失配，明文直接进入摘要开头。真实响应里键名大小写不固定（X-Auth-Token
// 是常见形态），故此性质是必需的而非锦上添花。
func TestRedactSnippet_CrossingWindow_CaseInsensitive(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"全大写", "TOKEN"},
		{"首字母大写", "Token"},
		{"连字符键大写", "X-Auth-Token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := []byte(`{"` + c.key + `":"` + strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`)

			got := RedactSnippet(body)

			if strings.Contains(got, "AAAA") {
				t.Errorf("大写敏感键 %q 跨粗截窗口时泄漏明文值，实际: %q", c.key, got)
			}
			// 掩码保留原始大小写，说明确由该键的残段命中而非碰巧被别的规则遮掉。
			if !strings.Contains(got, `"`+c.key+`":"***"`) {
				t.Errorf("大写敏感键 %q 跨窗后应被掩码且保留原写法，实际: %q", c.key, got)
			}
		})
	}
}

// TestRedactSnippet_CrossingWindow_WhitespaceAroundColon 锁定键名与冒号之间
// （以及冒号与值之间）的空白不破坏跨窗匹配。
//
// kvUnterminatedTailRe 在键名与冒号、冒号与值之间都是 \s*。把任一处收紧成
// 零空白，紧凑写法 "token" : " 的残段就会失配而泄漏；带换行的紧凑 JSON
// 更是格式化输出（缩进、换行）的常态。
func TestRedactSnippet_CrossingWindow_WhitespaceAroundColon(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"冒号两侧有空格", `{"token" : "` + strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`},
		{"冒号后有换行", `{"token":` + "\n" + `"` + strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`},
		{"冒号后有制表符", "{\"token\":\t\"" + strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactSnippet([]byte(c.body))

			if strings.Contains(got, "AAAA") {
				t.Errorf("带空白的敏感键值跨粗截窗口时泄漏明文，实际: %q", got)
			}
			if !strings.Contains(got, `"***"`) {
				t.Errorf("带空白的敏感键值跨窗后应被掩码，实际: %q", got)
			}
		})
	}
}

// TestRedactSnippet_CrossingWindow_CompleteKeyBeforeCrossingKey 锁定同一行内
// 先出现的完整敏感键不被跨窗键牵连，同时两个键都被正确掩码。
//
// 跨窗残段正则在值部分用的是 [^"]*：遇闭合引号即停，因此只锚定到文本末尾
// 的那一处残段。若把它放宽成贪婪的 .*，正则会从第一个键名一路吞到文本末尾，
// 把两个键之间的结构连同后一个键名一起替换掉——后一个键连名字都不复存在。
// 这条锁定的正是"只替换末尾残段、不动已完整闭合的键"。
func TestRedactSnippet_CrossingWindow_CompleteKeyBeforeCrossingKey(t *testing.T) {
	body := []byte(`{"token":"FIRSTSECRETVALUE","password":"` +
		strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`)

	got := RedactSnippet(body)

	if !strings.Contains(got, `"token":"***"`) {
		t.Errorf("跨窗前的完整敏感键应独立掩码，实际: %q", got)
	}
	if !strings.Contains(got, `"password":"***"`) {
		t.Errorf("跨窗的敏感键应被掩码且键名保留，实际: %q", got)
	}
	if strings.Contains(got, "FIRSTSECRETVALUE") {
		t.Errorf("完整敏感键的明文值不得泄漏: %q", got)
	}
	if strings.Contains(got, "AAAA") {
		t.Errorf("跨窗敏感键的明文值不得泄漏: %q", got)
	}
}

// TestRedactSnippetLen_CrossingWindow_Masked 锁定按指定长度的变体在跨窗时
// 同样不泄漏明文。
//
// 机制说明（此处曾有一处失实注释，订正如下）：该变体的安全**不依赖**
// clipPrefixWindow。实测把 RedactSnippetLen 改成绕过 clipPrefixWindow 直接
// 截原始字节，本用例仍然通过——因为 RedactBodyThenTruncate 内部是
// RedactBody 先掩码、再 256 截断、再按 max 截断，掩码本就发生在任何截断之前。
// 因此本用例只作跨窗形态的回归覆盖：真正锁住「先脱敏后截断」这条次序契约
// 的是同包的 TestRedactBodyThenTruncate_Order 与
// TestRedactSnippet_SensitiveValueCrossingTruncationBoundary（实测把
// RedactBodyThenTruncate 改成先截后掩时变红的是这两条，不是本条）。
func TestRedactSnippetLen_CrossingWindow_Masked(t *testing.T) {
	body := []byte(`{"token":"` + strings.Repeat("A", snippetPrefixWindow) + `","end":"x"}`)

	got := RedactSnippetLen(body, SnippetMaxLen)

	if strings.Contains(got, "AAAA") {
		t.Errorf("RedactSnippetLen 跨粗截窗口时泄漏明文值，实际: %q", got)
	}
	if !strings.Contains(got, `"token":"***"`) {
		t.Errorf("RedactSnippetLen 跨窗的敏感键应被掩码，实际: %q", got)
	}
}

// TestRedactSnippet_UnterminatedValueWithinWindow 锁定「body 本身残缺」的形态：
// 输入长度未达粗截窗口，但整个 body 就停在敏感值的值内部（无闭合引号）——
// 典型来源是服务端或反向代理超时把 JSON 截断在途中。
//
// 此前 clipPrefixWindow 只在「粗截切腰」这一条路径上补救残段，body 自身
// 残缺时原样返回，kvRe 因缺闭合引号失配，明文直接进入用户可见的错误摘要。
func TestRedactSnippet_UnterminatedValueWithinWindow(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		secret string
	}{
		{"token", `{"code":500,"msg":"timeout","data":{"token":"eyJhbGciOi.SUPERSECRETSIG`, "SUPERSECRETSIG"},
		{"password", `{"password":"p@ssw0rd-very-secret-value-abc`, "p@ssw0rd"},
		{"captcha", `{"captcha":"CAPTCHA_VALUE_XYZ`, "CAPTCHA_VALUE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactSnippet([]byte(c.body))
			if strings.Contains(got, c.secret) {
				t.Errorf("残缺 body 中的敏感值明文泄漏: %q", got)
			}
			if !strings.Contains(got, "***") {
				t.Errorf("残缺 body 的敏感值应产出掩码: %q", got)
			}
		})
	}
}

// TestRedactSnippet_ClosedValueWithinWindow 是上条的对照组：值已闭合时
// 走的是 kvRe 的常规路径，两条路径都必须不泄漏——但它们是不同的机制，
// 混在一起断言会掩盖任一条单独退化。
func TestRedactSnippet_ClosedValueWithinWindow(t *testing.T) {
	got := RedactSnippet([]byte(`{"token":"eyJhbGciOi.closed`))
	if strings.Contains(got, "closed") {
		t.Errorf("闭合的敏感值必须掩码: %q", got)
	}
}
