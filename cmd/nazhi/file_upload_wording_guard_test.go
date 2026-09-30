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
	"os"
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

// imagePrepFilePath 是 SDK 图片预处理实现，解码器注册表在其导入声明里。
const imagePrepFilePath = "../../pkg/client/image_prep.go"

// extractRegisteredImageFormats 用 AST 读 image_prep.go 的解码器注册导入，
// 得出真正被注册的图片格式（格式名取导入路径末段）。
//
// 为什么必须用 AST 而非正则：注册以 `_ "image/gif"` 这样的空导入形式出现，
// 正则一旦漏掉下划线或误匹配注释中的示例路径就会静默失配。
func extractRegisteredImageFormats(t *testing.T, path string) []string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("解析 %s 路径失败: %v", path, err)
	}
	fset := token.NewFileSet()
	f, perr := parser.ParseFile(fset, abs, nil, parser.ImportsOnly)
	if perr != nil {
		t.Fatalf("解析 %s 失败: %v", path, perr)
	}
	seen := make(map[string]bool)
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		// 图片格式的注册来源有两类，必须分别判据：
		//   - image/<fmt>（标准库）：包 init 自身调用 RegisterFormat，故普通
		//     导入即完成注册（image/jpeg 就是这样，不写空导入也生效）。
		//   - golang.org/x/image/<fmt>（扩展库）：只能靠空导入的 init 副作用。
		// 混用单一判据会漏项或误收：只认空导入会漏掉 jpeg（实测 4 个），
		// 只认路径前缀会把 image/color、image/draw 这类供代码使用的
		// 普通导入误计为格式（实测 7 个）。本守卫首次实现时凭直觉连错
		// 三次，均由期望值的正向断言当场报错。
		isStd := strings.HasPrefix(p, "image/")
		isExt := strings.HasPrefix(p, "golang.org/x/image/")
		if !isStd && !isExt {
			continue
		}
		// 扩展库必须是显式空导入（Name 非 nil 且为下划线；实测带 ImportsOnly
		// 解析时未具名的普通导入 Name 同样为 nil，故不能只判 nil）。
		// 标准库的 color、draw 是供代码使用的子包，不是解码器，按名单排除。
		if isExt && (imp.Name == nil || imp.Name.Name != "_") {
			continue
		}
		if isStd && (p == "image/color" || p == "image/draw" ||
			strings.HasPrefix(p, "image/color/") || strings.HasPrefix(p, "image/draw/")) {
			continue
		}
		if base := p[strings.LastIndex(p, "/")+1:]; base != "" {
			seen[base] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestImageDecoders_GodocMatchesRegistrations 锁定：SDK 上传 godoc 里
// 「已注册 xxx」那份格式枚举必须等于 image_prep.go 真实导入的解码器集合。
//
// 为什么 CLI 侧的守卫不够：TestFileUploadWording_CoveredByTruth 封死了
// 用户可见文案枚举图片格式，但 SDK godoc 同样写着「已注册 jpeg/png/gif/
// webp/bmp」。那串字面量今天恰好等于真实注册，可它与刚被诟病的 CLI 旧措辞
// 是同一类硬编码——将来 image_prep.go 增加或删除一个解码器导入，godoc 会
// 静默失实，而现有守卫只查 CLI 不查 SDK。
//
// 判据与结论相互独立：期望值是与提取函数无关的字面量表，提取逻辑出错时
// 不会连带把期望值一起改错。
func TestImageDecoders_GodocMatchesRegistrations(t *testing.T) {
	registered := extractRegisteredImageFormats(t, imagePrepFilePath)
	// 正向断言：提取器空转时守卫会恒绿，必须先确认真的扫到了东西。
	if len(registered) == 0 {
		t.Fatalf("未从 %s 提取到任何解码器注册，提取逻辑已失效", imagePrepFilePath)
	}

	// 独立字面量期望值，按书写序。增删 image_prep.go 的注册导入时必须同步
	// 更新本表，否则守卫断言的是过时契约。
	wantFormats := []string{"bmp", "gif", "jpeg", "png", "webp"}
	if len(registered) != len(wantFormats) {
		t.Fatalf("已注册解码器共 %d 个（%s），独立期望值是 %d 个（%s）。"+
			"注册表变化时必须同步更新本期望值",
			len(registered), strings.Join(registered, "/"),
			len(wantFormats), strings.Join(wantFormats, "/"))
	}
	for i, name := range wantFormats {
		if registered[i] != name {
			t.Fatalf("已注册解码器第 %d 项应为 %q，实际 %q（实际序列 %s）",
				i, name, registered[i], strings.Join(registered, "/"))
		}
	}

	// godoc 里那串「已注册 ...」枚举必须与期望表逐项一致。
	abs, err := filepath.Abs(clientFilePath)
	if err != nil {
		t.Fatalf("解析 %s 路径失败: %v", clientFilePath, err)
	}
	src, rerr := os.ReadFile(abs)
	if rerr != nil {
		t.Fatalf("读取 %s 失败: %v", abs, rerr)
	}
	godoc := string(src)
	marker := "已注册 "
	idx := strings.Index(godoc, marker)
	if idx < 0 {
		t.Fatalf("未在 %s 找到 %q 表述，godoc 措辞已变，守卫需同步更新", clientFilePath, marker)
	}
	// 按行取该句并 TrimRight 掉可能存在的 \r：Windows 上用脚本改写源文件
	// 会引入 CRLF，按 \n 截断会把 \r 带进断言消息。
	segment := ""
	for _, line := range strings.Split(godoc[idx:], "\n") {
		segment = strings.TrimRight(line, "\r")
		break
	}
	for _, name := range wantFormats {
		if !strings.Contains(segment, name) {
			t.Errorf("SDK 上传 godoc 的解码器枚举缺 %q，实际表述：%q", name, segment)
		}
	}
	// 反向：枚举里不得出现期望表之外的格式名（防止漏删已移除的解码器）。
	for _, bogus := range []string{"tiff", "heic", "svg", "avif"} {
		if strings.Contains(segment, bogus) {
			t.Errorf("SDK 上传 godoc 的解码器枚举含未注册格式 %q，实际表述：%q", bogus, segment)
		}
	}
}
