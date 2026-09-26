package types

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestNormalizeInteger_AcceptedForms 穷举平台可能返回的、应当被接受的形态。
// 这些形态此前散落七处实现且口径不一，收敛后由本测试单点锁定。
func TestNormalizeInteger_AcceptedForms(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int64
	}{
		{"JSON 整数", `5`, 5},
		{"JSON 负整数", `-5`, -5},
		{"零", `0`, 0},
		{"整值浮点", `4.0`, 4},
		{"整值浮点负数", `-4.0`, -4},
		{"科学计数整值", `1e3`, 1000},
		{"数字字符串", `"45"`, 45},
		{"数字字符串带空格", `" 1 "`, 1},
		{"负数字字符串", `"-5"`, -5},
		{"整值浮点字符串", `"5.0"`, 5},
		{"空串", `""`, 0},
		{"纯空格串", `"   "`, 0},
		{"null", `null`, 0},
		{"空字节", ``, 0},
		{"int64 上界", `9223372036854775807`, math.MaxInt64},
		{"int64 下界", `-9223372036854775808`, math.MinInt64},
		{"2^53+1（int64 快速路径保真）", `9007199254740993`, 9007199254740993},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeInteger(json.RawMessage(c.raw))
			if err != nil {
				t.Fatalf("NormalizeInteger(%s) 期望成功，得到错误: %v", c.raw, err)
			}
			if got != c.want {
				t.Fatalf("NormalizeInteger(%s) = %d，期望 %d", c.raw, got, c.want)
			}
		})
	}
}

// TestNormalizeInteger_RejectedForms 锁定必须拒绝的形态。
func TestNormalizeInteger_RejectedForms(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"非整值", `2.5`},
		{"负非整值", `-2.5`},
		{"非整值字符串", `"2.5"`},
		{"非数字文本", `"abc"`},
		{"布尔字面量", `true`},
		{"对象", `{}`},
		{"数组", `[]`},
		{"2^63（int64 溢出为负）", `9223372036854775808`},
		{"2^64", `18446744073709551616`},
		{"科学计数正向越界", `1e30`},
		{"科学计数负向越界", `-1e30`},
		{"越界整值字符串", `"9223372036854775808"`},
		{"越界科学计数字符串", `"1e30"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if v, err := NormalizeInteger(json.RawMessage(c.raw)); err == nil {
				t.Fatalf("NormalizeInteger(%s) 期望报错，实际放行并得到 %d", c.raw, v)
			}
		})
	}
}

// TestNormalizeInteger_OutOfRangeNeverWraps 锁定正向越界不被回绕放行。
//
// 旧写法 `v != float64(int64(v))` 的故障模式是：超过 int64 可表示范围的
// 整数字面量做往返转换时溢出回绕，回绕值恰好等于原值，判定通过，
// 随后以负数或 0 污染业务判断。本测试直接断言该故障模式不再出现。
func TestNormalizeInteger_OutOfRangeNeverWraps(t *testing.T) {
	for _, lit := range []string{
		"9223372036854775808",
		"9223372036854775809",
		"18446744073709551616",
		"1e19",
		"1e30",
	} {
		v, err := NormalizeInteger(json.RawMessage(lit))
		if err != nil {
			continue
		}
		t.Errorf("正向越界字面量 %s 不得放行，实际得到 %d", lit, v)
		if v < 0 {
			t.Errorf("越界字面量 %s 回绕成负数 %d，正是旧写法的故障模式", lit, v)
		}
	}
}

// TestNormalizeIntegerText_TextEntry 锁定纯文本入口的口径：TrimSpace 后解析，
// 空串归零，非整值与越界拒绝。
func TestNormalizeIntegerText_TextEntry(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "5", want: 5},
		{in: "  5  ", want: 5},
		{in: "-5", want: -5},
		{in: "5.0", want: 5},
		{in: "", want: 0},
		{in: "   ", want: 0},
		{in: "5.5", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "9223372036854775808", wantErr: true},
		{in: "1e30", wantErr: true},
	}
	for _, c := range cases {
		got, err := NormalizeIntegerText(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeIntegerText(%q) 期望报错，实际得到 %d", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeIntegerText(%q) 期望成功，得到错误: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeIntegerText(%q) = %d，期望 %d", c.in, got, c.want)
		}
	}
}

// TestNormalizeIntegerValue_CoversDecodedForms 锁定 map 解码路径的入口：
// json.Unmarshal 到 map[string]any 会把整数解成 float64，使用 UseNumber
// 时产出 json.Number，两条路径必须归一到同一结果。
func TestNormalizeIntegerValue_CoversDecodedForms(t *testing.T) {
	// 标准解码：整数变 float64。
	var m map[string]any
	if err := json.Unmarshal([]byte(`{"id":4383235}`), &m); err != nil {
		t.Fatalf("标准解码失败: %v", err)
	}
	if v, err := NormalizeIntegerValue(m["id"]); err != nil || v != 4383235 {
		t.Fatalf("标准解码路径期望 4383235，得到 %d（err=%v）", v, err)
	}

	// UseNumber 解码：产出 json.Number，应与标准路径同值。
	dec := json.NewDecoder(strings.NewReader(`{"id":4383235}`))
	dec.UseNumber()
	var m2 map[string]any
	if err := dec.Decode(&m2); err != nil {
		t.Fatalf("UseNumber 解码失败: %v", err)
	}
	if _, ok := m2["id"].(json.Number); !ok {
		t.Fatalf("UseNumber 路径应产出 json.Number，实际 %T", m2["id"])
	}
	if v, err := NormalizeIntegerValue(m2["id"]); err != nil || v != 4383235 {
		t.Fatalf("UseNumber 路径期望 4383235，得到 %d（err=%v）", v, err)
	}

	// 七位以上标识符必须保真——这正是 school_id 曾被误判为非数字的故障场景。
	dec3 := json.NewDecoder(strings.NewReader(`{"id":9007199254740993}`))
	dec3.UseNumber()
	var m3 map[string]any
	if err := dec3.Decode(&m3); err != nil {
		t.Fatalf("UseNumber 大数解码失败: %v", err)
	}
	if v, err := NormalizeIntegerValue(m3["id"]); err != nil || v != 9007199254740993 {
		t.Fatalf("UseNumber 大数路径期望 9007199254740993，得到 %d（err=%v）", v, err)
	}
}

// TestNormalizeIntegerValue_RejectsNonNumeric 锁定 map 路径的拒绝面。
func TestNormalizeIntegerValue_RejectsNonNumeric(t *testing.T) {
	for _, v := range []any{
		true,
		[]any{1},
		map[string]any{},
		struct{}{},
		"abc",
		2.5,
	} {
		if got, err := NormalizeIntegerValue(v); err == nil {
			t.Errorf("NormalizeIntegerValue(%#v) 期望报错，实际得到 %d", v, got)
		}
	}
}

// TestNormalizeIntegerValue_RejectsOutOfRangeFloat 锁定 map 路径的越界拒绝。
// float64 分支是旧写法曾静默回绕的入口。
func TestNormalizeIntegerValue_RejectsOutOfRangeFloat(t *testing.T) {
	for _, f := range []float64{
		math.MaxInt64, // 舍入后为 2^63
		1 << 63,       // 恰为 2^63
		1e30,
		-1e30,
		math.MaxInt64 * 2,
	} {
		if v, err := NormalizeIntegerValue(f); err == nil {
			t.Errorf("NormalizeIntegerValue(%v) 期望报错，实际放行得到 %d", f, v)
		}
	}
}

// TestNormalizeIntegerFloat_TruncDiscipline 直接锁定浮点入口的整值判定：
// 非整值必须拒绝，不得静默截断（4.7 被截成 4 会让按 code 反查名称命中错项）。
func TestNormalizeIntegerFloat_TruncDiscipline(t *testing.T) {
	if v, err := NormalizeIntegerFloat(4.0, "4.0"); err != nil || v != 4 {
		t.Fatalf("4.0 期望放行为 4，得到 %d（err=%v）", v, err)
	}
	if v, err := NormalizeIntegerFloat(4.7, "4.7"); err == nil {
		t.Fatalf("4.7 期望拒绝（不得截断为 4），实际放行得到 %d", v)
	}

	// 整值判定与范围判定是两道叠加的防线，各自独立生效：
	// 2^62 是合法整数（int64 往返不溢出，两种写法结果相同），
	// 必须放行——确认上界检查没有把合法大整数误伤。
	if v, err := NormalizeIntegerFloat(1<<62, "4611686018427387904"); err != nil || v != 1<<62 {
		t.Fatalf("2^62 期望放行为 %d，得到 %d（err=%v）", int64(1<<62), v, err)
	}
	// 恰好 2^63 是整值（Trunc 通过）但越界（范围判定拒绝），
	// 确认两道防线各自都不可省：只留 Trunc 会放行，只留范围会漏判非整值。
	if v, err := NormalizeIntegerFloat(1<<63, "9223372036854775808"); err == nil {
		t.Fatalf("2^63 期望被范围判定拒绝，实际放行得到 %d", v)
	}
	if v, err := NormalizeIntegerFloat(-4.7, "-4.7"); err == nil {
		t.Fatalf("-4.7 期望拒绝（不得截断为 -4），实际放行得到 %d", v)
	}
}
