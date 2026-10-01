package client

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync"

	"github.com/disintegration/imaging"
	// WEBP 解码器通过 image.RegisterFormat 注册，image.Decode 自动派发
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// MaxImageSize 默认压缩目标上限（5MB）。
const MaxImageSize = 5 * 1024 * 1024

// MinImageDimension 缩放下限（像素），低于此值停止缩放。
const MinImageDimension = 10

// qualityAfterOptimization 是缩放级联的 JPEG 降档质量。
// 主路径从 q92 起步（prepareImageForUpload 首次 encodeJPEG(img, 92)）；
// 仅当 q92 输出超限时才降档到本值重编一次，再超限走缩放级联。
const qualityAfterOptimization = 80

// ErrImageTooLarge 压缩后仍超过 MaxImageSize。
var ErrImageTooLarge = errors.New("image exceeds maximum size after compression")

// ErrUnsupportedFormat 不支持的图片格式。
var ErrUnsupportedFormat = errors.New("unsupported image format")

// prepareImageForUpload 读取本地图片，预处理为符合平台要求的 JPG 字节流。
//
// 流程：
//  1. decodeImage 读文件并按魔术字节识别格式
//  2. 解码 + 透明合成 + 动画取首帧
//  3. 编码为 JPG（q92 起步）
//  4. 超限则降档。**降档是有条件的链，不是无条件依次尝试**：
//     - q92 超 5MB 且未超 10MB（2×上限）→ q80 重编一次；仍超限则进缩放级联。
//     超 10MB 则**跳过 q80 档**直接进缩放级联（q80 对超大图通常不够，
//     省一次编码）。
//     - 缩放级联先试温和档 0.25 倍边长 + q80（短边需 ≥40px，否则整档跳过），
//     未达标再试极限档 0.082 倍边长 + q40（短边需 ≥122px；不足则跳过缩放，
//     以原尺寸 q40 编码）。
//     - 仍超限则 ErrImageTooLarge。
//     另有更早的失败出口：任一档编码失败即返回（带 ErrInvalidPayload），
//     并非「四档全落空后才报错」。
//
// 全部在内存中完成，不写盘、不修改原文件。
func (c *Client) prepareImageForUpload(path string) ([]byte, string, error) {
	img, err := decodeImage(path)
	if err != nil {
		return nil, "", err
	}

	// 透明合成：所有含透明通道的图片（NRGBA/RGBA/Paletted/GIF/NYCbCrA）都走 flattenOnWhite。
	//
	// GIF 透明区域若不合成白底，jpeg.Encode 会输出黑底；
	// flattenOnWhite 对 Paletted 一并处理。
	flattened := hasTransparency(img)
	if flattened {
		img = flattenOnWhite(img)
	}

	// 尝试用 92 起步
	data, err := encodeJPEG(img, 92)
	if err != nil {
		return nil, "", fmt.Errorf("JPG 编码失败: %w", errors.Join(ErrInvalidPayload, err))
	}

	// 已满足
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}

	// 如果 data 远超上限（>2×MaxImageSize），跳过质量级联（省一次 q80 encode），
	// 直接进缩放级联。quality=80 对超大图片通常不够降到 ≤5MB，缩放最少省 50% 体积。
	q := qualityAfterOptimization
	if len(data) > 2*MaxImageSize {
		goto scaleCascade
	}

	// 质量级联（只跑一次 quality=80）
	data, err = encodeJPEG(img, q)
	if err != nil {
		return nil, "", fmt.Errorf("质量 %d 编码失败: %w", q, errors.Join(ErrInvalidPayload, err))
	}
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}

	// 添加 scaleCascade 标签，质量级联跳过时直接跳入
scaleCascade:
	b := img.Bounds()

	// 温和档：缩到 25% + quality=80。覆盖 10~30MB 大图的常见区间，
	// 避免直接跳极限档导致输出只有几十 KB 的画质断崖。
	w25 := int(float64(b.Dx()) * 0.25)
	h25 := int(float64(b.Dy()) * 0.25)
	if w25 >= MinImageDimension && h25 >= MinImageDimension {
		mid, midErr := encodeJPEG(imaging.Resize(img, w25, h25, imaging.Lanczos), qualityAfterOptimization)
		if midErr == nil && len(mid) <= MaxImageSize {
			return mid, "image/jpeg", nil
		}
	}

	// 极限档：单次缩放取代 7 轮累乘：0.7^7 ≈ 0.082，避免 4K 图 ~200MB 临时内存。
	// ponytail: 单边 ≤121px 的极端长宽比图（如 80000×100 退化图像）会整体放弃缩放
	// 直接原尺寸 q40 编码——若仍超限返回 ErrImageTooLarge。真实相机/截图不可达此形态，
	// 需要支持时改为逐维钳制 max(finalW, MinImageDimension) 后必缩放。
	finalW := int(float64(b.Dx()) * 0.082)
	finalH := int(float64(b.Dy()) * 0.082)
	current := img
	if finalW >= MinImageDimension && finalH >= MinImageDimension {
		current = imaging.Resize(img, finalW, finalH, imaging.Lanczos)
	}

	// 统一编码为 JPG（quality=40），只 encode 一次
	data, err = encodeJPEG(current, 40)
	if err != nil {
		c.logDebug("缩放级联最终 encodeJPEG 失败：err=%v", err)
		return nil, "", fmt.Errorf("缩放级联编码失败: %w", errors.Join(ErrInvalidPayload, err))
	}
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}
	// 兜底：缩小到极限仍超限
	return nil, "", ErrImageTooLarge
}

// decodeImage 用 imaging.Decode 解码（内部即 image.Decode + EXIF 自动旋转），
// 按魔术字节派发到已注册的格式。
//
// 在线可达的格式来自依赖树而非本文件 import：本包显式注册 bmp/webp，
// 标准库 gif/jpeg/png，**另有 TIFF 经 imaging 的普通导入传递自注册**
// （golang.org/x/image/tiff 的 init 调 image.RegisterFormat，进程全局生效）。
// 故此处不逐一枚举格式——枚举清单一旦落后于依赖树就会失实。
// 各格式解码出的动态类型见 image_prep_transparency_guard_test.go。
//
// BMP 在解码失败后检测魔术字节，单独给出「请转为 PNG/JPG」的提示。
// 该提示携带的 ErrUnsupportedFormat 与压缩超限的 ErrImageTooLarge 同归参数档
// （400 / 退出码 3，见 cmd/nazhi 的 mapSentinelToHTTPCode）。
func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		// 本地 IO 错误归 ErrInvalidPayload（调用方可控输入问题）→ CLI 400/exit3。
		return nil, fmt.Errorf("打开图片失败: %w", errors.Join(ErrInvalidPayload, err))
	}
	defer f.Close()

	// AutoOrientation=true：JPEG 带 EXIF Orientation 时自动旋转摆正，对齐前端
	// canvas drawImage 的现代浏览器默认行为（竖拍照片上传后不再横置）。
	// 非 JPEG 或无 EXIF 时行为与 image.Decode 一致。
	img, err := imaging.Decode(f, imaging.AutoOrientation(true))
	if err != nil {
		// 标准 BMP 已由 x/image/bmp 注册解码；此处仅捕获不支持的 BMP 变体等失败，
		// 检测 BM 魔数给出友好提示
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr == nil {
			var magic [2]byte
			if _, readErr := io.ReadFull(f, magic[:]); readErr == nil && magic[0] == 'B' && magic[1] == 'M' {
				return nil, fmt.Errorf("%w: BMP（请先用图片工具转为 PNG/JPG）", ErrUnsupportedFormat)
			}
		}
		// 同款策略：本地解码失败（目录/损坏文件等调用方可控输入）包 ErrInvalidPayload
		// 让 CLI 漏斗归 400/exit3，而非 500/exit2 被脚本无限重试。
		return nil, fmt.Errorf("图片解码失败: %w", errors.Join(ErrInvalidPayload, err))
	}
	return img, nil
}

// hasTransparency 检测图片是否含透明通道。
//
// **本枚举表必须与「本包实际可解码出的带 alpha 类型」保持同步**：漏掉一种
// 类型，该类型图片的透明区会被 jpeg.Encode 丢弃并落为黑底。
//
// 这不是理论风险——commit c46fbda 就是在补 NYCbCrA 分支（有损 WebP
// VP8+ALPH 的解码产物），该缺陷曾随 v1.5.1 之前的版本上线。
//
// 判据来源是 Go 的 image.Decode 按**内容**决定返回类型（go doc image/png
// 明写「The type of Image returned depends on the PNG contents」），
// 这是实现细节而非稳定契约。image_prep_transparency_guard_test.go 以运行时
// 解码真实字节锁定该表的完备性，依赖升级引入新类型时会变红。
//
// 只列带 alpha 槽的类型：Gray/Gray16/YCbCr/CMYK 无 alpha 通道，命中与否都
// 不影响正确性，纳入会让不透明图片也白做一次全图合成。
func hasTransparency(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.RGBA64:
		return true
	case *image.Paletted:
		// GIF Paletted 几乎都有透明
		return true
	case *image.NYCbCrA:
		// 有损 WebP（VP8+ALPH）解码产物；不合成白底则 jpeg.Encode
		// 丢弃 alpha，透明区经预乘落为黑底（无损 VP8L 是 NRGBA，
		// 已被上方分支覆盖）
		return true
	}
	return false
}

// flattenOnWhite 将含透明通道的图片合成到白底 RGBA 图上。
func flattenOnWhite(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)
	return dst
}

// jpegBufPool 复用 bytes.Buffer 给 encodeJPEG，避免每次上传 5MB 图片时
// cascade 重编码 2-11 次的 buffer 重复分配/GC 压力。
// 注：bytes.Buffer.Get 出来必须 Reset；返回的 []byte 必须 copy（pool Put 后
// 内部 slice 会被其他 goroutine 复用覆盖）。
var jpegBufPool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

// encodeJPEG 编码为 JPG 字节流。
// 使用 sync.Pool 复用 buffer 减少 GC 压力，cascade 重编码场景下
// 5MB 图片多次 encode 共享同一个 buffer 实例。
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	buf, ok := jpegBufPool.Get().(*bytes.Buffer)
	if !ok || buf == nil {
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	defer func() {
		// 释放前清空，避免 buffer 持有对 img 像素的引用导致 GC 无法回收
		buf.Reset()
		jpegBufPool.Put(buf)
	}()
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	// 关键：必须 copy 出来再返回——pool Put 后 buffer 内部 slice 会被
	// 其他 goroutine 复用，buf.Bytes() 返回的引用会立刻失效
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}
