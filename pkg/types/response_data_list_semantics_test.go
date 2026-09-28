// response_data_list_semantics_test.go 锁定 DecodeDataList 的三条边界语义。
//
// 这三条此前只存在于多处字段旁的注释里（honor.go、flexjson.go、
// typical_case.go、circle.go、submitted.go 共 6 处生产注释），函数本身的
// godoc 零声明，也没有任何测试断言过——把 decodeFieldSlice 的返回
// `v, err` 改成 `nil, err` 后全包测试零变红，即行为完全未被守护。
//
// 本文件把注释里的知识变成可执行契约。
package types

import (
	"encoding/json"
	"testing"
)

// listProbeRec 用于制造「单个字段类型违约」，不参与任何真实 wire 形态断言。
type listProbeRec struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TestDecodeDataList_TypeViolationCollapsesWholePage 锁定：任一元素任一字段
// 类型违约即整页丢弃，不返回部分结果。
//
// 这是本包的主动选择而非 encoding/json 的固有性质——原生 Unmarshal 对
// 同一输入会保留已解出的元素并让违约元素取零值。
func TestDecodeDataList_TypeViolationCollapsesWholePage(t *testing.T) {
	raw := json.RawMessage(`[{"id":1,"name":"a"},{"id":"notanumber","name":"b"}]`)
	resp := UnifiedResponse{Code: 1, DataList: &raw}

	got, err := DecodeDataList[listProbeRec](resp)
	if err == nil {
		t.Fatal("字段类型违约应返回错误")
	}
	// 核心断言是「已解出的首元素被丢弃」，而非仅「返回了错」——
	// 任何返回 error 但保留部分结果的实现都会在这里被抓住。
	if got != nil {
		t.Errorf("整页语义要求丢弃全部已解出元素，实际保留 %+v", got)
	}
}

// TestDecodeDataList_TypeViolationCollapsesRegardlessOfPosition 锁定违约元素的
// 位置不影响结果，防止日后被改成「遇错即停、保留前缀」。
func TestDecodeDataList_TypeViolationCollapsesRegardlessOfPosition(t *testing.T) {
	raw := json.RawMessage(`[{"id":"notanumber","name":"a"},{"id":2,"name":"b"}]`)
	resp := UnifiedResponse{Code: 1, DataList: &raw}

	got, err := DecodeDataList[listProbeRec](resp)
	if err == nil {
		t.Fatal("首元素违约同样应返回错误")
	}
	if got != nil {
		t.Errorf("首元素违约时也不得保留后续元素，实际 %+v", got)
	}
}

// TestDecodeDataList_EmptyFormsAreNotErrors 锁定三种空形态的区分：
// 缺键与 null 归 nil 且不算错误；空数组返回非 nil 空切片。
//
// 非 nil 这一点是承重的：pkg/client/honor.go 的 returnData 兜底判据
// 依赖「缺失为 nil」与「空数组为非 nil 空切片」的可区分性。
func TestDecodeDataList_EmptyFormsAreNotErrors(t *testing.T) {
	t.Run("缺键", func(t *testing.T) {
		got, err := DecodeDataList[listProbeRec](UnifiedResponse{Code: 1})
		if err != nil {
			t.Fatalf("缺键不应算错误: %v", err)
		}
		if got != nil {
			t.Errorf("缺键应返回 nil，得到 %+v", got)
		}
	})

	t.Run("null 字面量", func(t *testing.T) {
		raw := json.RawMessage(`null`)
		got, err := DecodeDataList[listProbeRec](UnifiedResponse{Code: 1, DataList: &raw})
		if err != nil {
			t.Fatalf("null 不应算错误: %v", err)
		}
		if got != nil {
			t.Errorf("null 应返回 nil，得到 %+v", got)
		}
	})

	t.Run("空数组", func(t *testing.T) {
		raw := json.RawMessage(`[]`)
		got, err := DecodeDataList[listProbeRec](UnifiedResponse{Code: 1, DataList: &raw})
		if err != nil {
			t.Fatalf("空数组不应算错误: %v", err)
		}
		if got == nil {
			t.Error("空数组应返回非 nil 空切片，nil 会使调用方无法与「缺键」区分")
		}
		if len(got) != 0 {
			t.Errorf("空数组长度应为 0，实际 %d", len(got))
		}
	})
}
