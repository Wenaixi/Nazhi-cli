package types

import "testing"

func TestTaskLevelName_Exhaustive(t *testing.T) {
	cases := []struct{ code, want string }{
		{TaskLevelNational, "国家"},
		{TaskLevelProvince, "省"},
		{TaskLevelCity, "地区/市"},
		{TaskLevelCounty, "区/县/街道/社区"},
		{TaskLevelSchool, "校"},
		{TaskLevelGrade, "年段"},
		{"", ""},
		{"0", ""},
		{"7", ""},
		{"99", ""},
	}
	for _, tc := range cases {
		got := TaskLevelName(tc.code)
		if got != tc.want {
			t.Fatalf("TaskLevelName(%q) want %q got %q", tc.code, tc.want, got)
		}
	}
	if TaskLevelNational != "1" || TaskLevelGrade != "6" {
		t.Fatal("level constants drift")
	}
}

func TestCheckResultName_Exhaustive(t *testing.T) {
	cases := []struct{ code, want string }{
		{CheckResultExcellent, "优秀"},
		{CheckResultGood, "良"},
		{CheckResultPass, "合格"},
		{CheckResultPoor, "差"},
		{"", ""},
		{"0", ""},
		{"5", ""},
	}
	for _, tc := range cases {
		got := CheckResultName(tc.code)
		if got != tc.want {
			t.Fatalf("CheckResultName(%q) want %q got %q", tc.code, tc.want, got)
		}
	}
}

func TestLevelTables_MatchConstants(t *testing.T) {
	if len(TaskLevelNames) != 6 {
		t.Fatalf("TaskLevelNames 应有 6 项，实际 %d", len(TaskLevelNames))
	}
	if len(CheckResultNames) != 4 {
		t.Fatalf("CheckResultNames 应有 4 项，实际 %d", len(CheckResultNames))
	}
	if len(PlayRoleNames) != 3 {
		t.Fatalf("PlayRoleNames 应有 3 项，实际 %d", len(PlayRoleNames))
	}
	// 查表函数与表项不得脱节：函数只做一次 map 查表，理论上恒等，
	// 但改写函数实现（如加入大小写兼容或默认值）时，这层一致性必须由测试兜住。
	// 注意它防不住「改了表项的值」——值是否正确由本文件各组的穷举用例
	// 与 CLI 侧的字面量期望值锁定。
	for code := range TaskLevelNames {
		if got := TaskLevelName(code); got != TaskLevelNames[code] {
			t.Errorf("TaskLevelName(%q)=%q 与表项 %q 不一致", code, got, TaskLevelNames[code])
		}
	}
	for code := range CheckResultNames {
		if got := CheckResultName(code); got != CheckResultNames[code] {
			t.Errorf("CheckResultName(%q)=%q 与表项 %q 不一致", code, got, CheckResultNames[code])
		}
	}
	for code := range PlayRoleNames {
		if got := PlayRoleName(code); got != PlayRoleNames[code] {
			t.Errorf("PlayRoleName(%q)=%q 与表项 %q 不一致", code, got, PlayRoleNames[code])
		}
	}
}

func TestPlayRoleName_Exhaustive(t *testing.T) {
	cases := []struct{ code, want string }{
		{PlayRoleHost, "主持策划者"},
		{PlayRoleMainParticipant, "主要参与者"},
		{PlayRoleParticipant, "参与者"},
		{"", ""},
		{"0", ""},
		{"4", ""},
	}
	for _, tc := range cases {
		if got := PlayRoleName(tc.code); got != tc.want {
			t.Fatalf("PlayRoleName(%q) want %q got %q", tc.code, tc.want, got)
		}
	}
}
