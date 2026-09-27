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
func TestNoDirectStderrWritesInCmd(t *testing.T) {
	scanChannelWrites(t, channelScan{
		name:        "stderr",
		stdVar:      "Stderr",
		uniformExit: "warnToStderr",
		why:         "--quiet 承诺关闭所有 stderr 输出",
		// output_sink.go 是通道本身的实现，必须现取 os.Stderr（缓存指针会让
		// 既有猴补全局的测试读到空串，见该文件注释）。
		allowed: map[string]bool{"output_sink.go": true},
	})
}

// TestNoDirectStdoutWritesInCmd 是 stderr 守卫的 stdout 对称面。
//
// stdout 的纪律是「只承载成功数据，错误一律写 stderr」。该承诺历史上被违反
// 29 处——参数错误曾被劈成两个通道（printParamError 走 stderr 与
// printEnvelope(envelope.Error(...)) 走 stdout 两种出口并存），写操作骨架
// 六步里第 1 步与第 6 步走 stdout、中间四步走 stderr。此前只有 stderr 侧有
// 守卫，stdout 侧的回流无任何约束：新增一处 printEnvelope(envelope.Error(...))
// 不会被任何测试拦下。
func TestNoDirectStdoutWritesInCmd(t *testing.T) {
	scanChannelWrites(t, channelScan{
		name:        "stdout",
		stdVar:      "Stdout",
		uniformExit: "printEnvelope",
		why:         "stdout 只承载成功数据；错误信封写 stdout 会让脚本误判成功",
		// output_sink.go 是通道实现；completion.go 直写的是 shell 补全脚本
		// 而非 JSON envelope（塞进信封无意义），属有意直写。
		allowed: map[string]bool{"output_sink.go": true, "completion.go": true},
	})
}

// channelScan 描述一条输出通道的直写扫描配置。
type channelScan struct {
	// name 是通道名，用于断言文案。
	name string
	// stdVar 是 os 包下该通道的变量名（Stdout / Stderr）。
	stdVar string
	// uniformExit 是该通道应经的统一出口符号名。
	uniformExit string
	// why 是违规后果的一句话说明。
	why string
	// allowed 是允许直取该通道的文件白名单。
	allowed map[string]bool
}

// scanChannelWrites 扫描包内非测试文件，报告所有绕过统一出口的通道直写。
//
// 两种直写形态的接收者 AST 形状不同，必须分别判：
//
//  1. os.Stdout.Write(b) / os.Stderr.WriteString(s) —— 通道变量本身是
//     SelectorExpr，作为 call.Fun 的 X。若按「X 必须是包标识符 os」判断，
//     此形态会整类漏掉（X 是 SelectorExpr 而非 Ident），而它恰是本守卫
//     要拦的直写。
//  2. fmt.Fprintf(os.Stdout, ...) —— X 是包标识符 fmt，通道变量在首参。
//
// 判据只看「X 是否就是该通道变量」或「X 是否是 fmt 且首参是该通道变量」，
// 不对方法名附加条件：附加 sel.Sel.Name == 通道名 会让方法调用形态的条件
// 恒假（os.Stderr.Write 的方法名是 Write），整类漏网。
func scanChannelWrites(t *testing.T, cfg channelScan) {
	t.Helper()
	isTarget := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := sel.X.(*ast.Ident)
		return ok && x.Name == "os" && sel.Sel.Name == cfg.stdVar
	}

	fset := token.NewFileSet()
	var violations []string
	scanned := 0

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
		if cfg.allowed[filepath.Base(path)] {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			// 语法错误不属本守卫职责，交给编译器报错。
			return nil
		}
		scanned++
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if isTarget(sel.X) {
				violations = append(violations,
					fset.Position(call.Pos()).String()+": 直接对 os."+cfg.stdVar+" 调用 "+sel.Sel.Name)
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "fmt" &&
				len(call.Args) > 0 && isTarget(call.Args[0]) {
				violations = append(violations,
					fset.Position(call.Pos()).String()+": "+x.Name+"."+sel.Sel.Name+
						" 直写 os."+cfg.stdVar+"（应经 "+cfg.uniformExit+"）")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源码失败: %v", err)
	}

	// 正向断言：守卫必须确实扫到了文件。否则它在目录遍历失效时静默全绿
	// —— 历史上 filepath.Walk 以 root 自身的 basename 作首次回调名，传相对
	// 路径时那个 name 会命中 SkipDir 判据，整棵子树未被扫描而守卫仍然全绿。
	if scanned == 0 {
		t.Fatalf("%s 通道守卫未扫描到任何文件，守卫已静默失效", cfg.name)
	}

	if len(violations) > 0 {
		t.Errorf("发现 %d 处绕过 %s 的 %s 直写:\n  %s\n这会违反 %s。",
			len(violations), cfg.uniformExit, cfg.name,
			strings.Join(violations, "\n  "), cfg.why)
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
