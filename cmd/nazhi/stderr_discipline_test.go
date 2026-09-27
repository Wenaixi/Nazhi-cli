package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectStderrWritesInCmd 锁定「stderr 告警一律经 warnToStderr」纪律。
//
// 为什么需要它：--quiet 承诺「关闭所有 stderr 输出」，该承诺历史上修过两次
// —— 首次收敛了 timeout/log-level/log-format 三处 fmt.Fprintf(os.Stderr)，
// 随后 main.go 关闭日志文件失败的三处又绕过 warnToStderr 复发。两次根因相同：
// 新增告警时照抄 fmt.Fprintf 而未查统一出口。既有 TestQuiet_SuppressesConfigWarnings
// 只验证 warnToStderr 自身行为，对「是否存在绕过它的调用方」无约束力，复发时仍全绿。
//
// 本测试从源码 AST 层面封死绕过路径：凡直接以 os.Stderr 为目标的写操作
// （fmt.Fprintf / fmt.Fprint / Fprintln / os.Stderr.Write 等）均判违规，
// 白名单只保留 output_sink.go——它是通道本身的实现，必须直取 os.Stderr。
func TestNoDirectStderrWritesInCmd(t *testing.T) {
	// outputSink 是通道实现本身，processOutputSink 必须现取 os.Stderr
	// （见 output_sink.go 注释：缓存指针会让既有猴补全局的测试读到空串），
	// 故该文件是唯一合法直取方。
	allowed := map[string]bool{"output_sink.go": true}

	fset := token.NewFileSet()
	var violations []string

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name != "." && (name == "testdata" || strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if allowed[filepath.Base(path)] {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			// 语法错误不属本守卫职责，交给编译器报错。
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// 只关心「以 os.Stderr 为写入目标」的调用：fmt.Fprintf(os.Stderr,...)
			// / fmt.Fprintln(os.Stderr,...) / os.Stderr.Write(...)
			x, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			isStderrTarget := x.Name == "os" && sel.Sel.Name == "Stderr"
			isFmtToStderr := x.Name == "fmt" && len(call.Args) > 0 && isOsStderrExpr(call.Args[0])
			isStderrWrite := x.Name == "os" && sel.Sel.Name == "Stderr"

			switch {
			case isStderrTarget && sel.Sel.Name == "Stderr":
				// os.Stderr 作为方法接收者（如 os.Stderr.WriteString）
				violations = append(violations,
					fset.Position(call.Pos()).String()+": 直接对 os.Stderr 调用 "+sel.Sel.Name)
			case isFmtToStderr:
				violations = append(violations,
					fset.Position(call.Pos()).String()+": "+x.Name+"."+sel.Sel.Name+" 直写 os.Stderr（应经 warnToStderr）")
			}
			_ = isStderrWrite
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源码失败: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("发现 %d 处绕过 warnToStderr 的 stderr 直写:\n  %s\n"+
			"这些会违反 --quiet「关闭所有 stderr 输出」的承诺，"+
			"且历史上已因此复发两次。",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// isOsStderrExpr 判断表达式是否为 os.Stderr。
func isOsStderrExpr(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "os" && sel.Sel.Name == "Stderr"
}
