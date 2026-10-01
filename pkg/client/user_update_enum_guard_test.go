package client

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestUserEnumMaps_LockedToFrontend 锁定四张中文枚举表的逐项码值。
//
// 取值来源是前端参照源码 user/modifyBox.vue 的 el-option 清单与提交侧
// if-else 映射（性别 :48-49/:254-258、团员 :81-82/:260-264、
// 民族 :66-73/:267-283、证件类型 :29-35/:287-301）。
//
// 期望值必须是独立字面量，不得由被测 map 派生——否则「map 漂移 + 期望
// 一起漂」会使断言恒成立。
//
// 守卫的必要性：四张表全部零测试覆盖，而危险形态不是「新增一项」
// （新增时未命中即 ErrInvalidPayload，是安全失败），而是「改错已有码值」
// ——例如把「维吾尔族: 3」误改为 4，CLI 会把维吾尔族学生静默存成畲族，
// 属无声的用户数据损坏。
//
// 前端日后新增枚举项时本守卫会红，这是期望行为：提醒两处同步。
func TestUserEnumMaps_LockedToFrontend(t *testing.T) {
	cases := []struct {
		name string
		got  map[string]int
		want map[string]int
	}{
		{"genderMap", genderMap, map[string]int{"男": 1, "女": 2}},
		{"youthLeagueMap", youthLeagueMap, map[string]int{"是": 1, "否": 0}},
		{"nationMap", nationMap, map[string]int{
			"汉族": 1, "满族": 2, "维吾尔族": 3, "畲族": 4,
			"回族": 5, "壮族": 6, "土家族": 7, "苗族": 8,
		}},
		{"idCardTypeMap", idCardTypeMap, map[string]int{
			"中国居民身份证": 1, "外国人永久居留身份证": 2, "港澳居民来往内地通行证": 3,
			"港澳台居民居住证": 4, "台湾居民来往大陆通行证": 5, "护照": 6,
			"香港永久性居民身份证": 7,
		}},
	}

	for _, tc := range cases {
		// 正向断言：先确认表非空且含锚点键，避免「四张表全被清空成空 map」
		// 时逐项比对全部通过而守卫静默失效。
		if len(tc.got) == 0 {
			t.Errorf("%s 被清空，逐项比对会全部通过，守卫失效", tc.name)
			continue
		}
		if len(tc.got) != len(tc.want) {
			t.Errorf("%s 条目数 = %d, want %d（漏项与多项都要拦住）", tc.name, len(tc.got), len(tc.want))
		}
		for k, wantCode := range tc.want {
			gotCode, ok := tc.got[k]
			if !ok {
				t.Errorf("%s 缺少条目 %q（期望码值 %d）", tc.name, k, wantCode)
				continue
			}
			if gotCode != wantCode {
				t.Errorf("%s[%q] = %d, want %d", tc.name, k, gotCode, wantCode)
			}
		}
	}
}

// TestUserEnumMaps_AnchorKeysPresent 正向锚点断言：守卫自身有效性的前置检查。
// 四张表各自的代表项必须在，否则说明表被整体改名或清空。
func TestUserEnumMaps_AnchorKeysPresent(t *testing.T) {
	anchors := []struct {
		name string
		m    map[string]int
		key  string
	}{
		{"genderMap", genderMap, "男"},
		{"youthLeagueMap", youthLeagueMap, "是"},
		{"nationMap", nationMap, "汉族"},
		{"idCardTypeMap", idCardTypeMap, "中国居民身份证"},
	}
	for _, a := range anchors {
		if _, ok := a.m[a.key]; !ok {
			t.Errorf("%s 应含锚点键 %q，实际键集=%v", a.name, a.key, a.m)
		}
	}
}

// TestUserEnumMaps_SupportedHintDerivedFromTable 锁定拒绝分支的错误文案
// 携带合法值清单。
//
// 用户传 {"nationName":"汉"} 时只得到「不支持的民族值 "汉"」，无从得知应填
// 「汉族」；--help 与 payload 允许键清单都只列键名、不列取值域。这是用户
// （尤其是脚本与 AI 代理）唯一的排错线索。
//
// commit b58f5db 把 switch 改成 map 时删掉了性别与团员两处的「（支持：…）」
// 提示，且未在别处补偿。此处锁定提示存在且由表派生——文案与表不可能脱节。
func TestUserEnumMaps_SupportedHintDerivedFromTable(t *testing.T) {
	// 排序按 UTF-8 字节序（sort.Strings 的既定语义），非拼音序。
	if got, want := supportedValues(genderMap), "女/男"; got != want {
		t.Errorf("性别合法值清单 = %q, want %q", got, want)
	}
	if got, want := supportedValues(nationMap), "回族/土家族/壮族/汉族/满族/畲族/维吾尔族/苗族"; got != want {
		t.Errorf("民族合法值清单 = %q, want %q", got, want)
	}
	// 证件类型含中文与 ASCII 混排，字典序在 UTF-8 字节序下确定。
	if got := supportedValues(idCardTypeMap); !strings.Contains(got, "护照") ||
		strings.Count(got, "/") != len(idCardTypeMap)-1 {
		t.Errorf("证件类型合法值清单异常: %q", got)
	}
}

// TestUpdateMyInfoStructured_RejectionMentionsSupportedValues 端到端锁定：
// 拒绝未支持值时，错误文本必须同时含用户输入值与合法值清单。
func TestUpdateMyInfoStructured_RejectionMentionsSupportedValues(t *testing.T) {
	c := internalNewTestClient()

	for _, tc := range []struct {
		field string
		value string
		want  string
	}{
		{"性别", "未知", "女/男"},
		{"团员", "也许", "否/是"},
		{"民族", "汉", "汉族"},
		{"证件类型", "身份证", "护照"},
	} {
		var input types.UserUpdateInput
		switch tc.field {
		case "性别":
			input.GenderName = tc.value
		case "团员":
			input.YouthLeague = tc.value
		case "民族":
			input.NationName = tc.value
		case "证件类型":
			input.IdCardType = tc.value
		}
		err := c.UpdateMyInfoStructured(t.Context(), "tok", input)
		if err == nil {
			t.Errorf("%s=%q 应被拒绝，实际 nil", tc.field, tc.value)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, fmt.Sprintf("不支持的%s值", tc.field)) {
			t.Errorf("%s 拒绝文案应含字段名，实际: %s", tc.field, msg)
		}
		if !strings.Contains(msg, tc.want) {
			t.Errorf("%s 拒绝文案应列出合法值 %q（用户据此排错），实际: %s", tc.field, tc.want, msg)
		}
	}
}
