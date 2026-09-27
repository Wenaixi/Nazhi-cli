package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// 读命令（取一个对象）模式的共享实现。
//
// 写操作族有 runWriteOp、列表族有 circleListMode，唯独最常见的「取一个对象」
// 没有 owner：十七处 Run 回调各自内联同一套七步骨架——读 flag、校验、建
// 客户端、进度文案、调 SDK、空列表归一、输出。归一纪律尤其脆弱：nil 切片
// 直塞 Success 信封会输出 "data":null，jq '.data[]' 对 null 报错退出，脚本
// 作者无从预期；同仓五个写实元数据命令都有归一，荣誉三个下拉命令曾因漏掉
// 而真的输出过 "data":null。
//
// runReadOp 把这条纪律收进单点：调用方只声明「本命令的差异」，
// 「空即空数组」由构造保证，漏写不再可能。
//
// 与另两个模式的分工：runWriteOp 持写操作六步骨架，circleListMode 持列表
// 模式分叉与 count 分支，runReadOp 持单对象取数的校验与归一。三者形状相近
// 但正交，不合并。

// readOpMode 描述一个读命令的领域差异。
type readOpMode struct {
	// verboseMsg 是调用 SDK 前的进度文案，为空则不输出。
	verboseMsg string
	// errorPrefix 是 SDK 调用失败时的错误文案前缀。
	errorPrefix string

	// validate 在建客户端之前校验已读入的 flag，返回错误消息；
	// nil 表示该命令无本地校验。参数错误以 printParamError 拒绝（退出码 3），
	// 且不发任何业务请求。
	validate func(cmd *cobra.Command) error

	// fetch 调用 SDK 取数并交出结果。返回的列表若为 nil，由 runner
	// 归一为空数组后交给 success。
	fetch func(ctx context.Context, c *client.Client, token string) (any, error)

	// success 把取数结果包装为 CLI 响应信封。
	success func(result any) *envelope.Envelope
}

// runReadOp 执行读命令的完整控制流。
//
// 错误优先次序（与列表族一致，读命令族沿用此口径）：
//  1. 本地 flag 校验 → printParamError（参数错误，退出码 3）
//  2. buildBizClient 失败 → printParamError（参数错误）
//  3. SDK 调用失败 → printError（按哨兵映射退出码）
//
// 校验在建客户端之前：缺 --token 与 flag 非法同时发生时，用户先看到真正
// 该修的那个错，而不是被配置问题挡住。
func runReadOp(cmd *cobra.Command, mode readOpMode) {
	if mode.validate != nil {
		if err := mode.validate(cmd); err != nil {
			printParamError(err)
			return
		}
	}

	c, token, err := buildBizClient(cmd)
	if err != nil {
		printParamError(err)
		return
	}

	if mode.verboseMsg != "" {
		printVerbose(mode.verboseMsg)
	}

	result, err := mode.fetch(cmd.Context(), c, token)
	if err != nil {
		if mode.errorPrefix != "" {
			err = fmt.Errorf("%s: %w", mode.errorPrefix, err)
		}
		printError(err)
		return
	}

	// 空列表归一：nil 切片直塞 Success 信封会输出 "data":null，
	// 下游 jq '.data[]' 对 null 报错退出。此处单点保证「空即空数组」。
	result = normalizeEmptyList(result)

	printEnvelope(mode.success(result))
}

// normalizeEmptyList 把取数结果中的 nil 记录列表归一为空数组。
//
// 平台在无数据时可能返回 null 或空数组，Go 侧都解成 nil 切片；直接封进
// Success 信封会输出 "data":null，下游 jq '.data[]' 对 null 报错退出。
// 本函数是这条契约的唯一实现处。
//
// 判据是「是不是记录列表」，不是「是不是 nil 切片」。仅按 Kind==Slice
// 判断会把不透传的 json.RawMessage（底层同为 []byte）误当记录列表：nil
// RawMessage 会被换成非 nil 的零长 RawMessage，既让调用点的 == nil 判据
// 失效，又不是合法 JSON（MarshalJSON 返回空字节，编码器报 unexpected end
// of JSON input），信封序列化失败后 stdout 空白且退出码 1——比 data:null
// 更难排查。记录列表的元素是结构化对象，RawMessage 承载的是已序列化的
// 原始 JSON，两者据此分开。
//
// 早期版本用「枚举已知切片类型」实现，只覆盖 []map[string]any 与 []any，
// 而本仓读命令的返回值绝大多数是具名切片类型（[]types.HonorSelectOption、
// []types.Dimension、[]types.HonorType 等），一个都不命中——守卫收编了
// 用不上的类型，漏掉了真正需要它的类型，导致调用点各自手抄一份归一。
// 现按元素类型判定，新增返回类型自动受保护。
//
// 非 nil 的空列表原样返回：它本就序列化为 []，构造一个同类型新切片没有收益。
// 非列表载荷（含 nil RawMessage）原样返回，由调用方的 success 闭包按各自
// 载荷语义决定空形态——它们的口径互不相同，无全局规则可套。
func normalizeEmptyList(result any) any {
	if result == nil {
		return []map[string]any{}
	}
	rv := reflect.ValueOf(result)
	if !isRecordList(rv) || !rv.IsNil() {
		return result
	}
	// 保留原类型，只把 nil 换成同长度的空切片，
	// 这样调用方与断言看到的仍是它传进来的那个具名类型。
	empty := reflect.MakeSlice(rv.Type(), 0, 0)
	return empty.Interface()
}

// isRecordList 判断值是否为「记录列表」——元素是结构化对象的切片。
// 判据是元素类型而非容器类型：json.RawMessage 的容器是 []byte，但元素是
// 已序列化的原始 JSON，不属于记录列表。
func isRecordList(rv reflect.Value) bool {
	if rv.Kind() != reflect.Slice {
		return false
	}
	// 元素为字节的切片是「一串字节」而非「一串记录」，不适用空数组归一。
	return rv.Type().Elem().Kind() != reflect.Uint8
}

// validatePaginationFlags 校验分页参数合法性，是分页纪律的单一实现处。
//
// 三条规则与拒绝理由：
//   - 页码与页长必须为正整数：负值或零透传会发出 pageNo=-1 之类的异常请求；
//   - 页长上钳 maxPageSize：服务端单页上限 500，超限透传会被静默截断为 500，
//     分页脚本以错误的 pageSize 计算页数拿到截断数据却不自知。
//
// 上界文案由 maxPageSize 派生而非写死字面量：此前 honor list 与
// typical-case list 把 500 硬编码进错误文案，而 circle images 用常量格式化，
// 改常量时前两者会对用户谎报「不能超过 500」而实际按新值拒绝。
func validatePaginationFlags(page, pageSize int) error {
	if page <= 0 || pageSize <= 0 {
		return fmt.Errorf("--page 与 --page-size 必须为正整数")
	}
	if pageSize > maxPageSize {
		return fmt.Errorf("--page-size 不能超过 %d（服务端单页上限）", maxPageSize)
	}
	return nil
}

// readListSuccess 是读列表型命令共用的 success 包装：直接以取数结果为载荷。
func readListSuccess(result any) *envelope.Envelope {
	return envelope.Success(result)
}
