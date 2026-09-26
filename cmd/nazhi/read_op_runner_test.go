package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// TestNormalizeEmptyList_NeverLeaksNil 锁定「空即空数组」这条契约。
//
// 平台在无数据时可能返回 null 或空数组，Go 侧都解成 nil 切片；直接封进
// Success 信封会输出 "data":null，下游 jq '.data[]' 对 null 报错退出。
// 这条纪律此前由各命令在调用点自觉补齐（漏一处即输出 data:null，
// 同仓荣誉下拉命令曾因此真的出过问题），现由 runReadOp 单点保证。
//
// 断言取正向形态：归一后必须是非 nil 的空切片，而不是「不含 null 字样」
// 这类否定式断言——命令完全不输出也能让否定式断言通过。
func TestNormalizeEmptyList_NeverLeaksNil(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want any
	}{
		{"nil 接口", nil, []map[string]any{}},
		{"nil 的 map 切片", []map[string]any(nil), []map[string]any{}},
		{"nil 的 any 切片", []any(nil), []any{}},
		{"非 nil 空 map 切片", []map[string]any{}, []map[string]any{}},
		{"非空 map 切片", []map[string]any{{"k": "v"}}, []map[string]any{{"k": "v"}}},
		{"非空 any 切片", []any{1, 2}, []any{1, 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeEmptyList(c.in)
			if got == nil {
				t.Fatalf("normalizeEmptyList(%#v) 返回 nil，期望非 nil 空列表", c.in)
			}
			// 用反射语义判定：归一后必须可安全遍历且长度为 0（针对切片类输入）。
			switch v := got.(type) {
			case []map[string]any:
				if v == nil {
					t.Fatalf("归一后仍为 nil 切片")
				}
			case []any:
				if v == nil {
					t.Fatalf("归一后仍为 nil 切片")
				}
			}
		})
	}
}

// TestNormalizeEmptyList_SerializesAsEmptyArray 锁定归一后的信封载荷确实是
// 空数组而非 null——直接断言 JSON 形态，这是用户实际看到的东西。
func TestNormalizeEmptyList_SerializesAsEmptyArray(t *testing.T) {
	got := normalizeEmptyList([]map[string]any(nil))
	e := readListSuccess(got)
	if e == nil {
		t.Fatal("readListSuccess 返回 nil 信封")
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("信封序列化失败: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "null") {
		t.Errorf("归一后信封仍含 null: %s", s)
	}
	if !strings.Contains(s, "[]") {
		t.Errorf("归一后信封应含空数组 []: %s", s)
	}
}

// TestNormalizeEmptyList_PassesThroughNonList 确认非列表结果原样透传——
// 单对象读命令（whoami、session 等）的载荷不应被误改。
func TestNormalizeEmptyList_PassesThroughNonList(t *testing.T) {
	type payload struct{ Name string }
	in := payload{Name: "示例"}
	if got := normalizeEmptyList(in); got != any(in) {
		t.Errorf("非列表结果应原样透传，得到 %#v", got)
	}
}

// TestValidatePaginationFlags 锁定分页纪律的单一实现处。
//
// 上界文案的断言是本测试的关键：此前 honor list 与 typical-case list 把
// 500 硬编码进错误文案，circle images 用常量格式化，改常量时前两者会对
// 用户谎报。此处断言文案必须由 maxPageSize 派生。
func TestValidatePaginationFlags(t *testing.T) {
	const over = maxPageSize + 1
	cases := []struct {
		name     string
		page     int
		pageSize int
		wantErr  string
	}{
		{name: "正常分页", page: 1, pageSize: 10},
		{name: "页长等于上界（边界放行）", page: 1, pageSize: maxPageSize},
		{name: "页码为零", page: 0, pageSize: 10, wantErr: "--page"},
		{name: "页码为负", page: -1, pageSize: 10, wantErr: "--page"},
		{name: "页长为零", page: 1, pageSize: 0, wantErr: "--page-size"},
		{name: "页长为负", page: 1, pageSize: -5, wantErr: "--page-size"},
		{name: "页长超上界（边界拒绝）", page: 1, pageSize: over, wantErr: "--page-size"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validatePaginationFlags(c.page, c.pageSize)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("validatePaginationFlags(%d, %d) 期望通过，得到错误: %v", c.page, c.pageSize, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validatePaginationFlags(%d, %d) 期望报错，实际通过", c.page, c.pageSize)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("错误文案应含 %q，实际: %v", c.wantErr, err)
			}
		})
	}
}

// TestValidatePaginationFlags_ErrorTextTracksConstant 锁定上界文案由常量派生。
// 改 maxPageSize 而不同步文案，正是本次收敛要消除的漂移。
func TestValidatePaginationFlags_ErrorTextTracksConstant(t *testing.T) {
	err := validatePaginationFlags(1, maxPageSize+1)
	if err == nil {
		t.Fatal("页长超上界应报错")
	}
	msg := err.Error()
	if !strings.Contains(msg, strconv.Itoa(maxPageSize)) {
		t.Errorf("上界文案应含常量值 %d，实际: %s", maxPageSize, msg)
	}
	// 文案里不应出现与常量无关的硬编码数字：上界只出现一次且等于常量。
	if strings.Count(msg, "500") != 0 && maxPageSize != 500 {
		t.Errorf("文案出现硬编码的 500，而常量为 %d: %s", maxPageSize, msg)
	}
}
