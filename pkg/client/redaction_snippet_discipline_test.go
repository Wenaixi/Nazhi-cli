package client

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectRedactBodyThenTruncateInClient 锁定「诊断摘要一律经
// logx.RedactSnippet」纪律。
//
// 为什么需要它：摘要的脱敏次序与长度全部收在 RedactSnippet 内部，由它决定
// 调用方不需要知道「截多长」也不需要知道「先脱敏还是先截断」。仓库记忆写着
// 「七处调用点已全部收口」，但 httpDo 的响应体日志出口实际写的是
// logx.RedactBodyThenTruncate(respBytes, 100) —— 自带字面量长度且跳过了
// clipPrefixWindow。后果是当响应体自身在敏感值中间被截断时（服务端/反向
// 代理超时是常见来源），脱敏正则因缺闭合引号整体失配，明文进入日志；且
// levelForStatus 对 4xx→Warn、5xx→Error，CLI 默认级别即 warn，故默认配置下
// 每次错误响应都会走到这里。
//
// 与哨兵漏斗守卫（TestSentinelFunnelCoverage）同源的问题：纪律写在记忆里、
// 靠人工审查发现，本项目已因此复发三次（限读两处、哨兵两处、摘要一处）。
// 本测试把它变成结构性约束。
//
// 豁免范围：pkg/logx 自身（RedactSnippet 的实现内部就要调用它，
// 红actSnippetLen 亦然）。本文件在 pkg/client 包内，故扫描面是
// pkg/client 与 cmd/nazhi 两个包——它们是唯一可能从外部直调的两个地方。
func TestNoDirectRedactBodyThenTruncateInClient(t *testing.T) {
	forbidDirectSnippetCalls(t)
}

// forbidDirectSnippetCalls 扫描 pkg/client 与 cmd/nazhi 的非测试文件，
// 报告所有绕过 RedactSnippet 的摘要调用。
//
// 判据形态（与 stderr_discipline_test.go 的教训一致，两种形态都要判）：
//
//  1. logx.RedactBodyThenTruncate(b, n) —— call.Fun 是 SelectorExpr，
//     X 为包标识符 logx。
//  2. RedactBodyThenTruncate(b, n) —— 同包内以裸函数名调用（pkg/logx 内部，
//     本守卫不扫该包，此处保留分支以防将来扫描面扩大时漏网）。
//
// 只对 X 是包标识符做判定是有意的：若同时匹配方法调用形态
// （sel.X 本身是 SelectorExpr），会把未来某个同名方法误报。
func forbidDirectSnippetCalls(t *testing.T) {
	t.Helper()

	fset := token.NewFileSet()
	var violations []string
	scanned := 0

	// 扫描面固定为两个包：pkg/client（SDK 内所有摘要出口）与 cmd/nazhi
	// （CLI 侧若自行打摘要）。用绝对路径而非相对路径——filepath.Walk 以 root
	// 自身的 basename 作首次回调名，传 ".." 时那个 name 会命中隐藏目录的
	// SkipDir 判据，整棵子树未被扫描而守卫仍然全绿（见 mustAbs 的用法说明）。
	roots := []string{
		mustAbs(filepath.Join("..", "client")),
		mustAbs(filepath.Join("..", "..", "cmd", "nazhi")),
	}

	// 逐根计数，供下方「每根都必须扫到文件」的正向断言使用。
	scannedPerRoot := make(map[string]int, len(roots))
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if name := info.Name(); name == "testdata" || (strings.HasPrefix(name, ".") && name != ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				// 语法错误不属本守卫职责，交给编译器。
				return nil
			}
			scanned++
			scannedPerRoot[root]++
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr:
					if x, ok := fn.X.(*ast.Ident); ok && x.Name == "logx" &&
						fn.Sel.Name == "RedactBodyThenTruncate" {
						violations = append(violations,
							": 直调 logx.RedactBodyThenTruncate（诊断摘要应经 logx.RedactSnippet）")
					}
				case *ast.Ident:
					if fn.Name == "RedactBodyThenTruncate" {
						violations = append(violations,
							fset.Position(call.Pos()).String()+
								": 直调 RedactBodyThenTruncate（诊断摘要应经 RedactSnippet）")
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("遍历源码失败: %v", err)
		}
	}

	// 正向断言：守卫必须确实扫到了文件。缺了它，目录遍历一旦失效就是静默全绿
	// ——本项目已因 filepath.Walk 的 basename 判据踩过一次。
	if scanned == 0 {
		t.Fatal("摘要出口守卫未扫描到任何文件，守卫已静默失效")
	}
	// 正向断言必须逐根校验而非只看总数：本仓已踩过「目录遍历守卫静默
	// 失效」的坑（filepath.Walk 以 root 的 basename 作首次回调名，传
	// 相对路径时命中隐藏目录判据，整棵子树未被扫描而守卫仍然全绿）。若
	// 只判 scanned != 0，cmd/nazhi 那半扫描面失效时 pkg/client 自身
	// 仍会让总数非零，守卫照样绿。
	for _, root := range roots {
		if scannedPerRoot[root] == 0 {
			t.Errorf("摘要出口守卫在 %s 未扫描到任何文件，该侧扫描面已静默失效",
				root)
		}
	}

	if len(violations) > 0 {
		t.Errorf("发现 %d 处绕过 logx.RedactSnippet 的摘要调用:\n  %s\n"+
			"RedactSnippet 内部会用 clipPrefixWindow 收口「响应体自身在敏感值中间被截断」"+
			"的形态，直调 RedactBodyThenTruncate 会让该形态的明文进入日志。",
			len(violations), strings.Join(violations, "\n  "))
	}
}
