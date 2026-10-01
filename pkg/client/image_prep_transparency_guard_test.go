package client

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"github.com/disintegration/imaging"
	// 与 image_prep.go 的 import 保持一致：这些包通过 init() 向
	// image.RegisterFormat 自注册解码器，image.Decode 按魔术字节派发。
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// 本文件锁定 hasTransparency 的类型枚举表必须与「本包实际可解码出的、
// 且带 alpha 通道的」图片类型保持同步。
//
// 为什么需要守卫：Go 的 image.Decode 按**内容**决定返回的具体类型
// （go doc image/png 原文：「The type of Image returned depends on the PNG
// contents.」），这是一条实现细节而非稳定契约。hasTransparency 枚举了 6 种
// 类型，漏掉一种 = 透明区经 jpeg.Encode 落为黑底。
//
// 这不是理论风险：commit c46fbda 就是在补 NYCbCrA 分支（有损 WebP
// VP8+ALPH 的解码产物），该缺陷曾随 v1.5.1 之前的版本上线，而当时只补了
// 一条 NYCbCrA 的单点测试，没有任何穷举守卫。
//
// 守卫的判据是「运行时解码真实字节」而非「扫描 import 块推导格式清单」：
// 后者会漏掉传递依赖自注册的格式（TIFF 正是经 imaging 的普通导入传递注册，
// 不在本包 import 块里），也猜不出某格式实际返回哪种色彩模型。
// 带 alpha 通道的 image.Image 动态类型全集。
//
// 手工枚举自 stdlib 的 image/color.go 与 ycbcr.go：任何带 A 通道的类型都
// 必须被 hasTransparency 命中，否则其透明区会被丢弃。
//
// 只列「有 alpha 槽」的类型是必要的收窄：Gray/Gray16/YCbCr/CMYK 等无 alpha
// 通道的类型命中与否都不影响正确性——把它们纳入会让守卫对无害改动也报警。
//
// 与 sampleBackedTypes 的区别：RGBA/RGBA64 虽是带 alpha 类型，但 Go 标准库
// 的 png.Encode 对这两种色彩模型产出的是 NRGBA（cbTCA8 而非 cbTC8），
// 无法用标准库编码器造出「解码后得到 RGBA」的样本。它们仍必须被枚举
// （第三方编码器或依赖升级可能产出），故留在本清单里，只是不进样本集。
var alphaImageTypes = []string{
	"*image.NRGBA",
	"*image.NRGBA64",
	"*image.RGBA",
	"*image.RGBA64",
	"*image.NYCbCrA",
	"*image.Paletted",
}

// sampleBackedTypes 是本轮样本能实际解码出的带 alpha 类型子集。
// 期望集与运行时观察集的双向吻合只对这组断言——RGBA/RGBA64 因标准库
// 编码器不产出对应色彩模型而无样本，由「alphaImageTypes ⊆ 枚举集」
// 这条断言单独兜住。
var sampleBackedTypes = []string{
	"*image.NRGBA",
	"*image.NRGBA64",
	"*image.NYCbCrA",
	"*image.Paletted",
}

// enumeratedTransparencyTypes 用 AST 提取 hasTransparency 的 type switch
// 里枚举的全部类型名（形如 *image.NRGBA）。
//
// 判据经实测 dump 确认：case 元素是 *ast.StarExpr，其 .X 是
// *ast.SelectorExpr（.X 为包标识符 Ident，.Sel 为类型名 Ident）。
// 没有实测就写 AST 匹配是本仓反复踩过的坑——提取不到时若缺少正向
// 断言，守卫会静默恒绿。
func enumeratedTransparencyTypes(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "image_prep.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 image_prep.go 失败: %v", err)
	}

	var got []string
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "hasTransparency" {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range ts.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range cc.List {
					star, ok := expr.(*ast.StarExpr)
					if !ok {
						continue
					}
					sel, ok := star.X.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					pkg, ok := sel.X.(*ast.Ident)
					if !ok {
						continue
					}
					got = append(got, "*"+pkg.Name+"."+sel.Sel.Name)
				}
			}
			return false
		})
	}
	return got
}

// decodeViaProduction 走生产同形链路（imaging.Decode + AutoOrientation(true)），
// 返回解码结果的动态类型名。
func decodeViaProduction(data []byte) string {
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return ""
	}
	return typeNameOf(img)
}

func typeNameOf(img image.Image) string {
	switch img.(type) {
	case *image.NRGBA:
		return "*image.NRGBA"
	case *image.NRGBA64:
		return "*image.NRGBA64"
	case *image.RGBA:
		return "*image.RGBA"
	case *image.RGBA64:
		return "*image.RGBA64"
	case *image.NYCbCrA:
		return "*image.NYCbCrA"
	case *image.Paletted:
		return "*image.Paletted"
	case *image.Gray:
		return "*image.Gray"
	case *image.Gray16:
		return "*image.Gray16"
	case *image.YCbCr:
		return "*image.YCbCr"
	case *image.CMYK:
		return "*image.CMYK"
	default:
		return "unknown"
	}
}

// encodeSamples 为每种格式构造带 alpha 的真实字节样本。
// 返回的 map 是「格式名 → 字节」，解码后应命中 alphaImageTypes 中的类型。
func encodeSamples(t *testing.T) map[string][]byte {
	t.Helper()
	samples := map[string][]byte{}

	// PNG：标准库编码器覆盖多种色彩模型（cbTCA8/cbTC8 等）。
	for name, img := range map[string]image.Image{
		"png/nrgba":    image.NewNRGBA(image.Rect(0, 0, 4, 4)),
		"png/nrgba64":  image.NewNRGBA64(image.Rect(0, 0, 4, 4)),
		"png/rgba":     image.NewRGBA(image.Rect(0, 0, 4, 4)),
		"png/rgba64":   image.NewRGBA64(image.Rect(0, 0, 4, 4)),
		"png/paletted": image.NewPaletted(image.Rect(0, 0, 4, 4), color4()),
		"png/gray":     image.NewGray(image.Rect(0, 0, 4, 4)),
		"png/gray16":   image.NewGray16(image.Rect(0, 0, 4, 4)),
	} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("构造 %s 样本失败: %v", name, err)
		}
		samples[name] = buf.Bytes()
	}

	// GIF：唯一返回类型是 Paletted。
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, image.NewPaletted(image.Rect(0, 0, 4, 4), color4()), nil); err != nil {
		t.Fatalf("构造 gif 样本失败: %v", err)
	}
	samples["gif/paletted"] = gifBuf.Bytes()

	// JPEG：Gray 与 YCbCr 均无 alpha 通道，用于验证「无 alpha 类型不要求被命中」。
	for name, img := range map[string]image.Image{
		"jpeg/ycbcr": image.NewYCbCr(image.Rect(0, 0, 4, 4), image.YCbCrSubsampleRatio420),
		"jpeg/gray":  image.NewGray(image.Rect(0, 0, 4, 4)),
	} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, nil); err != nil {
			t.Fatalf("构造 %s 样本失败: %v", name, err)
		}
		samples[name] = buf.Bytes()
	}

	// WebP：Go 无 WebP 编码器，有损 VP8+ALPH 的解码产物是 *image.NYCbCrA，
	// 标准库编码器造不出该样本，故固化一份真实字节。
	//
	// 这条样本承载 c46fbda 修复的那类缺陷（有损 WebP 透明区落黑底），
	// 是本守卫唯一覆盖该路径的样本——删掉它 NYCbCrA 就退回「只被枚举、
	// 从未被运行时验证」的状态。
	webpData, err := os.ReadFile("testdata/lossy-alpha.webp")
	if err != nil {
		t.Fatalf("读取 testdata/lossy-alpha.webp 失败: %v", err)
	}
	samples["webp/nycbcra"] = webpData

	return samples
}

// color4 返回 4 色调色板。
func color4() color.Palette {
	return color.Palette{
		color.RGBA{R: 0, A: 0},
		color.RGBA{R: 85, A: 85},
		color.RGBA{R: 170, A: 170},
		color.RGBA{R: 255, A: 255},
	}
}

// TestHasTransparency_CoversEveryDecodableAlphaType 锁定枚举表的完备性：
// 凡是本包能解码出的带 alpha 通道图片，其动态类型都必须被 hasTransparency
// 判为含透明。
//
// 这是运行时断言而非源码扫描：样本经生产同形链路（imaging.Decode +
// AutoOrientation）解码，拿到真实动态类型后与枚举比对。
func TestHasTransparency_CoversEveryDecodableAlphaType(t *testing.T) {
	// 正向断言：枚举集非空且与期望集一致，防止比对整体空转。
	enumerated := enumeratedTransparencyTypes(t)
	if len(enumerated) == 0 {
		t.Fatal("hasTransparency 未枚举任何类型，提取判据可能已失效")
	}

	// 确认期望集本身是「已覆盖」的——若某类型既不在枚举也不在样本里，
	// 说明样本构造不足，守卫会失去发现新类型的能力。
	for _, want := range alphaImageTypes {
		found := false
		for _, got := range enumerated {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("alphaImageTypes 含 %q，但 hasTransparency 未枚举它——两份清单已漂移", want)
		}
	}

	// 核心断言：样本解码出的带 alpha 类型必须全部被枚举覆盖。
	samples := encodeSamples(t)
	if len(samples) == 0 {
		t.Fatal("样本集为空，守卫会静默通过")
	}

	// 无 alpha 通道的类型（命中与否都不影响正确性），记录用于自陈覆盖边界。
	noAlpha := map[string]bool{
		"*image.Gray": true, "*image.Gray16": true,
		"*image.YCbCr": true, "*image.CMYK": true,
	}

	// 记录本轮实际观察到的带 alpha 类型，用于把期望集钉在运行时事实上。
	observed := map[string]bool{}
	checked := 0
	for name, data := range samples {
		img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
		if err != nil {
			t.Errorf("样本 %s 应能解码，实际失败: %v", name, err)
			continue
		}
		gotType := typeNameOf(img)
		if gotType == "unknown" {
			t.Errorf("样本 %s 的动态类型 %T 未被 typeNameOf 识别；"+
				"新增类型时须同步 typeNameOf 与 alphaImageTypes，否则守卫会漏判", name, img)
			continue
		}
		if noAlpha[gotType] {
			continue
		}
		observed[gotType] = true
		if !hasTransparency(img) {
			t.Errorf("样本 %s 解码为 %s（带 alpha 通道），hasTransparency 应判为含透明；"+
				"漏判将使透明区经 jpeg.Encode 落为黑底", name, gotType)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("没有任何带 alpha 的样本被检查，守卫已失去发现能力")
	}

	// 期望集必须与运行时观察到的类型双向吻合。缺这一条时，把期望集删空
	// 即可让守卫静默通过——那只剩「枚举非空」一条弱断言在起作用。
	for _, want := range sampleBackedTypes {
		if !observed[want] {
			t.Errorf("期望集列了 %q，但本轮样本未解码出该类型：期望集已与运行时脱节"+
				"（清空或伪造期望集会让守卫失去发现新类型的能力）", want)
		}
	}
	// 反向：运行时若观察到期望集之外的带 alpha 类型，说明 stdlib 或依赖
	// 新增了色彩模型，必须显式决定是否纳入枚举，否则守卫对新类型是瞎的。
	for got := range observed {
		known := false
		for _, want := range sampleBackedTypes {
			if got == want {
				known = true
				break
			}
		}
		if !known {
			t.Errorf("运行时解码出新的带 alpha 类型 %s，但它不在 sampleBackedTypes 中；"+
				"须确认它是否需要 hasTransparency 命中，否则透明区会落黑底", got)
		}
	}
}

// TestHasTransparency_NoAlphaTypesNotRequired 记录并锁定守卫的收窄边界：
// 无 alpha 通道的类型即使未被枚举，也不构成缺陷（命中与否都不影响正确性）。
//
// 该用例存在的意义是防止有人日后把 Gray/YCbCr 也加进枚举「求全面」——
// 那会让 hasTransparency 对不透明图片也做一次全图白底合成，纯属无谓开销。
func TestHasTransparency_NoAlphaTypesNotRequired(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 4, 4))
	if hasTransparency(gray) {
		t.Error("Gray 无 alpha 通道，不应判为含透明（否则对不透明图做无谓的全图合成）")
	}
	ycbcr := image.NewYCbCr(image.Rect(0, 0, 4, 4), image.YCbCrSubsampleRatio420)
	if hasTransparency(ycbcr) {
		t.Error("YCbCr 无 alpha 通道，不应判为含透明")
	}
}
