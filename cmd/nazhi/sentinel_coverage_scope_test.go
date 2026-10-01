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

// sentinelDeclPattern 匹配「包级哨兵错误声明」：var/const 块里
// `ErrXxx = errors.New(...)` 的形态。与 errors.go 的形态相同。
//
// 该形态识别覆盖 pkg/client 的 errors.go 与 image_prep.go——两处都用
// `var ErrXxx = errors.New(...)` 声明导出的哨兵。
func isSentinelDeclName(name string) bool {
	return strings.HasPrefix(name, "Err") && len(name) > 3
}

// extractAllPackageSentinels 扫描 pkg/client 全部非测试文件，
// 提取所有包级哨兵错误声明名。
//
// 与既有 extractSentinels（只扫 errors.go）的差别是本文件的核心：
// image_prep.go 声明的 ErrImageTooLarge / ErrUnsupportedFormat 同样是
// 供调用方 errors.Is 判别的导出哨兵，却因不在 errors.go 而被漏斗守卫
// 排除在检查之外——它们至今未登记进 mapSentinelToHTTPCode。
func extractAllPackageSentinels(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "pkg", "client")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("解析 pkg/client 失败: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("未解析到任何包，扫描判据可能已失效")
	}

	found := map[string]string{}
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || (gen.Tok != token.VAR && gen.Tok != token.CONST) {
					continue
				}
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, ident := range vs.Names {
						if !isSentinelDeclName(ident.Name) {
							continue
						}
						// 值必须是 errors.New(...) 或同形态的构造器调用。
						isErrorCtor := false
						for _, v := range vs.Values {
							call, ok := v.(*ast.CallExpr)
							if !ok {
								continue
							}
							sel, ok := call.Fun.(*ast.SelectorExpr)
							if !ok {
								continue
							}
							pkgID, ok := sel.X.(*ast.Ident)
							if ok && pkgID.Name == "errors" {
								isErrorCtor = true
							}
						}
						if isErrorCtor {
							found[ident.Name] = filepath.Base(name)
						}
					}
				}
			}
		}
	}
	return found
}

// TestSentinelFunnelCoverage_AllPackageFiles 扩展既有哨兵覆盖守卫的覆盖面。
//
// 背景：既有 TestSentinelFunnelCoverage 只从 errors.go 提取哨兵，而
// image_prep.go 另声明了两个导出哨兵——
//
//	ErrImageTooLarge      压缩后仍超限
//	ErrUnsupportedFormat  不支持的图片格式（BMP 变体）
//
// 它们同样供调用方 errors.Is 判别，却因不在 errors.go 而从未被检查。
// 二者最终都被 file.go / decodeImage 包装成上传错误，经 CLI 漏斗时全部
// errors.Is 不命中，落 default: 500 → envelope 映射退出码 2。
//
// exit 2 是「服务端或网络故障、可退避重放」档。对「换个文件格式即可」
// 的本地问题报这一档，脚本会对永不成功的请求无限退避。
func TestSentinelFunnelCoverage_AllPackageFiles(t *testing.T) {
	declared := extractAllPackageSentinels(t)
	// 正向断言：提取器必须真的扫到了东西，否则整条守卫会静默通过。
	if len(declared) == 0 {
		t.Fatal("未从 pkg/client 提取到任何哨兵，提取逻辑已失效")
	}

	mapped := funnelReferencedSentinels(t)
	if len(mapped) == 0 {
		t.Fatal("未从 mapSentinelToHTTPCode 提取到任何哨兵引用，提取逻辑已失效")
	}

	missing := make([]string, 0, len(declared))
	for name, file := range declared {
		if mapped[name] {
			continue
		}
		if reason, ok := intentionalDefault500[name]; ok {
			if reason == "" {
				t.Errorf("哨兵 %s 在豁免表中但未写理由，等同绕过守卫", name)
			}
			continue
		}
		missing = append(missing, name+"("+file+")")
	}

	if len(missing) > 0 {
		t.Errorf("以下哨兵未进入 mapSentinelToHTTPCode，也未在豁免表写明理由：%v\n"+
			"它们会被漏斗判为 default 500 → envelope 映射退出码 2（可退避重放档）。\n"+
			"若确实应落 500，须在 intentionalDefault500 写明理由", missing)
	}
}

// TestSentinelFunnelCoverage_CoversImagePrepSentinels 点名断言：确保本守卫
// 的扩展确实覆盖到 image_prep.go 的两个哨兵。若将来它们被移走，
// 本用例会提示更新提取范围，而不是让守卫悄悄退回只扫 errors.go。
func TestSentinelFunnelCoverage_CoversImagePrepSentinels(t *testing.T) {
	declared := extractAllPackageSentinels(t)
	for _, want := range []string{"ErrImageTooLarge", "ErrUnsupportedFormat"} {
		file, ok := declared[want]
		if !ok {
			t.Errorf("应能从 pkg/client 提取到哨兵 %s（image_prep.go 声明），"+
				"提取范围可能已失效；若它被移走请同步更新本守卫", want)
			continue
		}
		if file != "image_prep.go" {
			t.Errorf("哨兵 %s 预期声明于 image_prep.go，实际在 %s", want, file)
		}
	}
}
