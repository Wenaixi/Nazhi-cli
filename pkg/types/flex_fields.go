package types

import (
	"encoding/json"
	"fmt"
)

// normalizeHoursField 是 hours 字段的 number/string 双类型兼容统一实现。
//
// Task / TaskCircleTypeInfo / CircleRecord 三个类型的 hours 字段此前各自
// 内联同一段 alias + Hours RawMessage + FlexFloat 委派骨架（仅类型名与
// 错误前缀不同）。收为本 helper 后，三处各一行调用；新增带 hours 的
// 类型无需再抄第三份。
func normalizeHoursField(raw json.RawMessage, name string) (float64, error) {
	if raw == nil {
		return 0, nil
	}
	var value FlexFloat
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s.hours: %w", name, err)
	}
	return value.Float64(), nil
}

// UnmarshalJSON 为 Task 的 hours 提供 number/string 双类型兼容，同时保持公开字段为 float64。
func (t *Task) UnmarshalJSON(data []byte) error {
	type taskAlias Task
	aux := struct {
		Hours json.RawMessage `json:"hours"`
		*taskAlias
	}{taskAlias: (*taskAlias)(t)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	hours, err := normalizeHoursField(aux.Hours, "Task")
	if err != nil {
		return err
	}
	t.Hours = hours
	return nil
}

// UnmarshalJSON 为 TaskCircleTypeInfo 的 hours 提供 number/string 双类型兼容。
func (t *TaskCircleTypeInfo) UnmarshalJSON(data []byte) error {
	type taskCircleTypeInfoAlias TaskCircleTypeInfo
	aux := struct {
		Hours json.RawMessage `json:"hours"`
		*taskCircleTypeInfoAlias
	}{taskCircleTypeInfoAlias: (*taskCircleTypeInfoAlias)(t)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	hours, err := normalizeHoursField(aux.Hours, "TaskCircleTypeInfo")
	if err != nil {
		return err
	}
	t.Hours = hours
	return nil
}

// UnmarshalJSON 为 CircleRecord 的 hours 提供 number/string 双类型兼容。
func (c *CircleRecord) UnmarshalJSON(data []byte) error {
	type circleRecordAlias CircleRecord
	aux := struct {
		Hours json.RawMessage `json:"hours"`
		*circleRecordAlias
	}{circleRecordAlias: (*circleRecordAlias)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	hours, err := normalizeHoursField(aux.Hours, "CircleRecord")
	if err != nil {
		return err
	}
	c.Hours = hours
	return nil
}
