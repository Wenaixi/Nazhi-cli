package logx

import (
	"regexp"
	"strings"
)

// 敏感 key 集合，大小写不敏感。
var sensitiveKeys = map[string]bool{
	"token":         true,
	"x-auth-token":  true,
	"authorization": true,
	"password":      true,
	"passwd":        true,
	"captcha":       true,
}

func isSensitiveKey(k string) bool {
	return sensitiveKeys[strings.ToLower(strings.TrimSpace(k))]
}

// maskValue 对敏感值做掩码，保留前后 2 字符便于排错。
func maskValue(v string) string {
	if len(v) <= 4 {
		return "***"
	}
	return v[:2] + "***" + v[len(v)-2:]
}

// RedactHeader 对 header 值脱敏，敏感 key 时掩码；Referer 中的 token 查询串也做掩码。
func RedactHeader(k, v string) string {
	if isSensitiveKey(k) {
		return maskValue(v)
	}
	if strings.EqualFold(k, "Referer") && tokenQueryRe.MatchString(v) {
		return tokenQueryRe.ReplaceAllString(v, `${1}=***`)
	}
	return v
}

// 匹配 JSON 中敏感 key 的值，大小写不敏感。
var kvRe = regexp.MustCompile(`(?i)"(token|x-auth-token|authorization|password|passwd|captcha)"\s*:\s*"[^"]*"`)

// 匹配 URL 查询串中的敏感参数：token 与 userName（学号属 PII，SECURITY.md 守卫范围）。
// 组1=键名（不含等号），组2=值；替换为 ${1}=*** 保留参数名便于排障。
var tokenQueryRe = regexp.MustCompile(`(?i)((?:x-auth-)?token|username|user_name)=([^&\s"]+)`)

// RedactBody 对 body 中的敏感 JSON 键值做掩码，对 URL token 查询串也掩码，并截断到 256 字符。
func RedactBody(s string) string {
	red := tokenQueryRe.ReplaceAllString(s, `${1}=***`)
	red = kvRe.ReplaceAllStringFunc(red, func(m string) string {
		idx := strings.Index(m, ":")
		if idx < 0 {
			return m
		}
		return m[:idx+1] + `"***"`
	})
	if len(red) > 256 {
		red = red[:256] + "..."
	}
	return red
}

// RedactBodyThenTruncate 先脱敏再截断。
// 旧顺序（logSafeBody 先裸截 100 字节再 RedactBody）会让跨截断边界的敏感值
// 因正则失配而泄漏前缀；本函数保证截断窗口内的敏感内容全部先被掩码。
// max 为截断上限；RedactBody 内部另有 256 字符硬上限，先执行保证幂等。
func RedactBodyThenTruncate(body []byte, max int) string {
	s := RedactBody(string(body))
	if len(s) > max {
		return s[:max]
	}
	return s
}

// 诊断摘要的长度上限。
//
// 此前该值由各调用点以字面量 100 传入 redactSnippet / RedactBodyThenTruncate，
// 散落八处且 interface 上不可见——改长度要逐点修改，漏一处即出现同一份错误
// 在不同出口长度不一致的漂移。收进模块后由这里单点持有。
const SnippetMaxLen = 100

// 摘要前的原始字节粗截窗口。
//
// 粗截的是原始字节前缀而非脱敏后的文本：摘要最终只保留 SnippetMaxLen 字符，
// 若先对整个响应体（例如 4MB）做 string() 分配与两遍全量正则，代价与最终
// 产出不成比例。安全性依赖「窗口远大于摘要上限」——窗口内的敏感值一定被
// 完整匹配并掩码，窗口外的字节根本不会进入输出。
const snippetPrefixWindow = 4096

// RedactSnippet 把响应体归一为脱敏后的诊断摘要。
//
// 这是错误消息附带诊断摘要的唯一入口：调用方不再决定长度、不再决定脱敏
// 与截断的先后顺序，只声明「我要一份诊断摘要」。
func RedactSnippet(body []byte) string {
	if len(body) > snippetPrefixWindow {
		body = body[:snippetPrefixWindow]
	}
	return RedactBodyThenTruncate(body, SnippetMaxLen)
}

// RedactSnippetLen 是按指定长度取诊断摘要的变体，供确需不同长度的调用方
// 使用；默认路径应走 RedactSnippet。
func RedactSnippetLen(body []byte, max int) string {
	if len(body) > snippetPrefixWindow {
		body = body[:snippetPrefixWindow]
	}
	return RedactBodyThenTruncate(body, max)
}

// RedactValue 按 key 判断是否需掩码。
func RedactValue(key, val string) string {
	if isSensitiveKey(key) {
		return maskValue(val)
	}
	return val
}
