// file_upload_wording_guard_test.go 守卫：file upload 的用户可见文案
// （--help 的 Long 与 --file 的 usage）必须与 SDK 真相源 directUploadExtensions
// 保持一致，不允许 CLI 侧另抄一份附件白名单。
//
// 为什么需要它——上传文案已有四份副本，其中两份各自漂移过：
//   - SDK godoc 曾漏 .pdf（用户 2026-08-25 明确要求支持 PDF，commit f1a28d1
//     只加了 map 项与测试，没同步 godoc 清单）
//   - CLI --file usage 把 png/jpg/jpeg/bmp 与附件白名单并列为一张
//     「支持的扩展名」清单，暗示图片也是封闭白名单，而实际分派是
//     「扩展名命中附件白名单则直传，否则一律按图片解码」
//
// 文案漂移的代价不同于代码漂移：白名单是用户可见契约，删一项即让用户
// 传一个看似合法的文件被拒，且无任何报错提示扩展名不支持。
//
// 为什么不从 SDK 导出白名单：directUploadExtensions 是 map，导出它会把
// 「无序集合」变成公开 API 面（调用方无法假定顺序），属破坏性面扩张
// （pkg/ 已历 4 次 BREAKING），而守卫用 AST 从源码提取即可拿到同一份
// 集合、零 API 代价。CLI 侧对 SDK 源码路径的读取已有先例
// （sentinel_coverage_guard_test.go 读 ../../pkg/client/errors.go）。
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// clientFilePath 是 SDK 上传实现，附件白名单真相源在其 directUploadExtensions 声明。
const clientFilePath = "../../pkg/client/file.go"

// extractDirectUploadExtensions 用 AST 提取 directUploadExtensions 的键。
//
// 不用正则：声明可能换行、加注释或改初始化形式，正则会静默失配——
// 这正是第七轮总结的「AST 守卫的匹配形态必须实测 dump，不能凭直觉写」。
func extractDirectUploadExtensions(t *testing.T, path string) []string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("解析 %s 路径失败: %v", path, err)
	}
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, abs, nil, 0)
	if perr != nil {
		t.Fatalf("解析 %s 失败: %v", path, perr)
	}

	var keys []string
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
				if name.Name != "directUploadExtensions" || i >= len(vs.Values) {
					continue
				}
				cl, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					t.Fatalf("directUploadExtensions 应为复合字面量，实际 %T", vs.Values[i])
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						t.Fatalf("白名单元素应均为键值对，实际 %T", elt)
					}
					lit, ok := kv.Key.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("白名单键应均为字符串字面量，实际 %T", kv.Key)
					}
					keys = append(keys, strings.Trim(lit.Value, `"`))
				}
			}
		}
	}
	// 排序让比对与报错输出稳定；提取器刻意不依赖源码声明顺序。
	sort.Strings(keys)
	return keys
}

// fileUploadWordingTargets 返回需要与白名单比对的文案及其所在位置描述。
//
// 从命令树反查而不是硬编码文案字符串，将来 file upload 换实现位置也不会
// 让本守卫静默退化成空转。
func fileUploadWordingTargets() []struct{ where, text string } {
	usage := ""
	if f := fileUploadCmd.Flags().Lookup("file"); f != nil {
		usage = f.Usage
	}
	return []struct{ where, text string }{
		{"--file usage", usage},
		{"Long", fileUploadCmd.Long},
	}
}

// TestFileUploadWording_CoveredByTruth 锁定：附件白名单每一项都出现在
// file upload 的两处用户可见文案中，且文案不得出现白名单外的附件格式。
func TestFileUploadWording_CoveredByTruth(t *testing.T) {
	whitelist := extractDirectUploadExtensions(t, clientFilePath)
	// 正向断言：提取器必须真的扫到了东西。提取逻辑空转时守卫会恒绿，
	// 本仓已有两次「守卫静默失效且全绿」的教训（filepath.Walk SkipDir、
	// errors.Is AST 形态猜错），故每侧提取都须自带非空断言。
	if len(whitelist) == 0 {
		t.Fatalf("未从 %s 提取到任何附件白名单项，提取逻辑已失效", clientFilePath)
	}

	// 期望清单必须是独立字面量：若改成 extractDirectUploadExtensions 的返回值
	// 直接当期望，提取逻辑出错时两边一起错、断言恒成立。
	// 顺序按 CLI 文案书写序排列。
	wantList := []string{".pdf", ".mp4", ".txt", ".doc", ".docx", ".wps", ".rar", ".zip"}

	if len(whitelist) != len(wantList) {
		t.Fatalf("附件白名单共 %d 项（%s），本测试的独立期望值是 %d 项（%s）。"+
			"白名单增删时必须同步更新本期望值，否则守卫断言的是过时契约",
			len(whitelist), strings.Join(whitelist, "/"),
			len(wantList), strings.Join(wantList, "/"))
	}
	for _, target := range fileUploadWordingTargets() {
		if target.text == "" {
			t.Errorf("file upload 的 %s 文案为空，提取前提失效", target.where)
			continue
		}
		// 1. 白名单每一项（去掉前导点，与文案书写形态一致）必须出现。
		for _, ext := range whitelist {
			trimmed := strings.TrimPrefix(ext, ".")
			if !strings.Contains(target.text, trimmed) {
				t.Errorf("file upload 的 %s 缺附件格式 %q：%q", target.where, ext, target.text)
			}
		}
	}

	// 3. usage 不得再把图片格式与附件清单并列为「支持的扩展名」。
	//    判据独立于结论：白名单里没有任何图片格式，故凡出现
	//    png/jpg/jpeg/bmp/gif/webp 字样，即为把图片当封闭白名单枚举。
	imageExtLiterals := []string{"png", "jpg", "jpeg", "bmp", "gif", "webp"}
	for _, target := range fileUploadWordingTargets() {
		for _, img := range imageExtLiterals {
			if strings.Contains(target.text, img) {
				t.Errorf("file upload 的 %s 仍枚举图片格式 %q。"+
					"分派实际是「扩展名命中附件白名单则直传，否则一律按图片解码」，"+
					"图片格式不是白名单而是 image 解码器注册表，枚举会随依赖变化失实：%q",
					target.where, img, target.text)
			}
		}
	}
}
