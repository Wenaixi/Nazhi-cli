package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// 平台数值归一：把平台可能返回的多种数值形态收敛为整数。
//
// 平台对同一个字段可能返回 JSON number、整值浮点、数字字符串（含空格）、
// 空串或 null。同一份「怎样才算合法整数」的判定此前散落七处实现
// （FlexInt、parseFlexInt、parseFlexInt64、flexStringFromNumber、
// honorMapInt64、firstInt64、PayloadPositiveIDValid），防护程度各不相同：
// 旧写法 `v != float64(int64(v))` 在超过 int64 可表示范围的整数字面量上
// 会因 float 到 int64 往返溢出回绕而恰好相等，造成静默的错误解码。
// 本模块是这条知识的唯一实现处，调用方只声明「字段是否允许缺省」。
//
// 两条 interface 覆盖两类客观不同的输入形态：
//   - NormalizeInteger：结构化解码路径的原始 JSON 字节
//   - NormalizeIntegerValue：map 解码路径已得到的 any 值
//
// 两者的判定口径完全一致：整值判定、int64 范围拒绝、数字串去空格。

// NormalizeInteger 把平台的原始 JSON 字节归一为 int64。
//
// 接受的形态：JSON number、整值浮点（4.0）、数字字符串（含前后空格）、
// 空串与 null（均归零）。拒绝的形态：非整值（2.5）、超出 int64 可表示
// 范围的整数字面量（2^63 及以上）、以及无法解析的文本。
//
// 归零不等于放行：调用方若不允许字段缺省，需自行对零值做业务判定
// （如 PayloadPositiveIDValid 的正数要求），本模块不承载业务语义。
// 调用方需区分「键缺失」与「显式 null」时，应在调用归一前先判断
// json.RawMessage 是否为 nil——本模块收到的空字节一律归零。
func NormalizeInteger(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return 0, nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return 0, fmt.Errorf("解析数字字符串失败: %w", err)
		}
		return NormalizeIntegerText(s)
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return 0, fmt.Errorf("期望整数，得到 %s: %w", string(data), err)
	}
	if i, err := n.Int64(); err == nil {
		return i, nil
	}
	f, err := n.Float64()
	if err != nil {
		return 0, fmt.Errorf("无法解析数值 %q: %w", n.String(), err)
	}
	return NormalizeIntegerFloat(f, n.String())
}

// NormalizeIntegerText 把数字字符串归一为 int64，前后空格会被去除，
// 空串归零。无法解析或非整值时报错。
func NormalizeIntegerText(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("期望整数，得到 %q: %w", s, err)
	}
	return NormalizeIntegerFloat(f, s)
}

// NormalizeIntegerFloat 把浮点归一为 int64。非整值与超出 int64 可表示
// 范围的数值一律拒绝，避免静默截断或溢出回绕。
//
// 范围判定用 >= 而非 >：float64(math.MaxInt64) 舍入后恰等于 2^63，
// 因此 2^63 整数字面量必须拒绝，否则 int64 转换溢出为负值。
func NormalizeIntegerFloat(f float64, text string) (int64, error) {
	if f != math.Trunc(f) {
		return 0, fmt.Errorf("期望整数，得到 %v", f)
	}
	if f >= float64(math.MaxInt64) || f < float64(math.MinInt64) {
		return 0, fmt.Errorf("数值超出 int64 范围 %q", text)
	}
	return int64(f), nil
}

// NormalizeIntegerValue 把 map 解码路径已得到的 any 值归一为 int64。
//
// 覆盖 map[string]any 解码可能产出的全部数值形态：int、int32、int64、
// float32、float64、json.Number，以及数字字符串。与 NormalizeInteger
// 的区别只是入参已是解码后的值而非原始字节。
func NormalizeIntegerValue(v any) (int64, error) {
	switch n := v.(type) {
	case nil:
		return 0, nil
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case float32:
		return NormalizeIntegerFloat(float64(n), strconv.FormatFloat(float64(n), 'g', -1, 32))
	case float64:
		return NormalizeIntegerFloat(n, strconv.FormatFloat(n, 'g', -1, 64))
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, nil
		}
		f, err := n.Float64()
		if err != nil {
			return 0, fmt.Errorf("无法解析数值 %q: %w", n.String(), err)
		}
		return NormalizeIntegerFloat(f, n.String())
	case string:
		return NormalizeIntegerText(n)
	default:
		return 0, fmt.Errorf("期望整数，得到 %T", v)
	}
}
