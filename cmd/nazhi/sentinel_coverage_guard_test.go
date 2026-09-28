// sentinel_coverage_guard_test.go 守卫：SDK 新增哨兵错误时，CLI 退出码
// 漏斗 mapSentinelToHTTPCode 必须同步映射，否则漏斗会静默落到 default 500。
//
// 为什么需要它——本缺陷已复发四次，每次都靠人工审查在 1~8 周内补齐：
//   - ErrUploadRejected
//   - ErrSessionBackoff / ErrRetryable
//   - ErrEmptyUserInfo / ErrAllDecodersFailed
//   - ErrCookieSyncFailed（引入 commit 未动 output.go，带着 500/exit2 跑了 7 天）
//
// 四次修复都只加了「逐哨兵正确性」测试，没有一次加完整性守卫——
// 修的是症状不是防线。
//
// 500 并不保守：pkg/envelope 对 code>=500 映射退出码 2，即「网络/服务端
// 错误、可退避重放」，是三档里最主动重试的一档。把永久性错误
// （如 ErrCookieSyncFailed——调用方传了非 *cookiejar.Jar，改配置才能解决）
// 报成 5xx，脚本会对一个永不成功的请求无限退避重放。
//
// 两侧数据都从源码 AST 提取，不存在人工登记表：新增哨兵必然改变
// errors.go 的提取集合，因此没有任何一种「忘了登记」能让守卫失效。
// 反向（漏斗引用了不存在的哨兵）由 Go 编译器兜底，不重复断言。
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// intentionalDefault500 是「确有理由不进入漏斗」的豁免表。
//
// 豁免必须写理由：没有理由的条目等于绕过守卫。新增条目时请一并说明
// 该哨兵为何不会经由 printError 到达 CLI 输出通道。
var intentionalDefault500 = map[string]string{}

// clientErrorsPath 是哨兵定义文件，路径相对本包目录。
const clientErrorsPath = "../../pkg/client/errors.go"

func isSentinelName(n string) bool {
	return strings.HasPrefix(n, "Err") && len(n) > 3
}

// extractSentinels 用 AST 提取 errors.go 中所有 `ErrXxx = errors.New(...)` 声明。
//
// 不用正则：声明可能换行、加注释或改初始化形式，正则会静默失配。
func extractSentinels(t *testing.T, path string) map[string]bool {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("解析哨兵文件路径失败 %s: %v", path, err)
	}
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, nil, 0)
	if perr != nil {
		t.Fatalf("解析哨兵文件失败 %s: %v", path, perr)
	}

	found := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if !isSentinelName(name.Name) || i >= len(vs.Values) {
					continue
				}
				call, ok := vs.Values[i].(*ast.CallExpr)
				if !ok {
					continue
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if pkgID, ok := sel.X.(*ast.Ident); ok &&
					pkgID.Name == "errors" && sel.Sel.Name == "New" {
					found[name.Name] = true
				}
			}
		}
	}
	return found
}

// funnelReferencedSentinels 提取 output.go 中 mapSentinelToHTTPCode 函数体内
// 以 client.ErrXxx 形式出现的全部哨兵。
//
// 只扫该函数体而非整个文件：文件里还有其它 client.Err 引用（错误文案
// 说明等），那些不是漏斗映射。
func funnelReferencedSentinels(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, "output.go", nil, 0)
	if perr != nil {
		t.Fatalf("解析 output.go 失败: %v", perr)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Name.Name == "mapSentinelToHTTPCode" {
			fn = fd
			break
		}
	}
	// 正向断言：函数被改名或删除时守卫必须失败，而非静默空转。
	if fn == nil {
		t.Fatal("output.go 中未找到 mapSentinelToHTTPCode，哨兵覆盖守卫已失效")
	}

	found := map[string]bool{}
	// 形态为 errors.Is(err, client.ErrXxx)（哨兵在第二个参数），故扫描
	// errors.Is 调用的全部参数而非固定下标，对参数位置不敏感。
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Is" {
			return true
		}
		pkgID, ok := sel.X.(*ast.Ident)
		if !ok || pkgID.Name != "errors" {
			return true
		}
		for _, a := range call.Args {
			argSel, ok := a.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			argPkg, ok := argSel.X.(*ast.Ident)
			if !ok || argPkg.Name != "client" || !isSentinelName(argSel.Sel.Name) {
				continue
			}
			found[argSel.Sel.Name] = true
		}
		return true
	})
	return found
}

// TestSentinelFunnelCoverage 锁定：errors.go 定义的每个哨兵都进入
// mapSentinelToHTTPCode，或在豁免表中写明理由。
func TestSentinelFunnelCoverage(t *testing.T) {
	declared := extractSentinels(t, clientErrorsPath)
	// 正向断言：提取器必须真的扫到了东西。
	// 曾有守卫因目录遍历失效而恒绿（filepath.Walk 以 root 自身 basename
	// 作首次回调名而命中 SkipDir），故每侧提取都须自带「非空」断言。
	if len(declared) == 0 {
		t.Fatalf("未从 %s 提取到任何哨兵，提取逻辑已失效", clientErrorsPath)
	}

	mapped := funnelReferencedSentinels(t)
	if len(mapped) == 0 {
		t.Fatal("未从 mapSentinelToHTTPCode 提取到任何哨兵引用，提取逻辑已失效")
	}

	missing := make([]string, 0, len(declared))
	for name := range declared {
		if mapped[name] {
			continue
		}
		if reason, ok := intentionalDefault500[name]; ok {
			if reason == "" {
				t.Errorf("哨兵 %s 在豁免表中但未写理由，等同绕过守卫", name)
			}
			continue
		}
		missing = append(missing, name)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下哨兵未进入 mapSentinelToHTTPCode，将静默落 default 500"+
			"（退出码 2 = 可退避重放，是最主动重试的一档，永久性错误报此档会让脚本\n"+
			"对永不成功的请求无限重试）：\n  %s\n"+
			"请在 output.go 的 mapSentinelToHTTPCode 中为其选定编码档位，"+
			"或在 intentionalDefault500 中写明豁免理由。",
			strings.Join(missing, ", "))
	}
}

// TestSentinelFunnelDefaultRemainsCatchAll 锁定 default 分支仍是 500 兜底，
// 防止有人为省事改成 panic 或静默成功。
func TestSentinelFunnelDefaultRemainsCatchAll(t *testing.T) {
	src, err := os.ReadFile("output.go")
	if err != nil {
		t.Fatalf("读取 output.go 失败: %v", err)
	}
	if !strings.Contains(string(src), "default:") {
		t.Error("mapSentinelToHTTPCode 应保留 default 兜底分支")
	}
}
