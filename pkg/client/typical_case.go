package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 典型案例下拉展示名取自 pkg/types 的三张对照表（该处为唯一真相源）。
// 用户只选 code；SDK 在 *Name 为空时自动补全。
// type=2 为「社会调查报告」（非「社会实践报告」）；level=1 为「国际」（非写实列表的「国家」）。
//
// 表移入 pkg/types 的理由：写实域三表同址，CLI 已有 nazhi task level-codes
// 据此输出查表命令；典型案例三表原先留在本包未导出，CLI 无法枚举，用户
// 无从核对 payload 里 type/role/level 的合法取值。查表命令分设两路
// （nazhi typical-case level-codes），不与写实域并入同一信封——两套 level
// 码表语义不同，合在一起正是 CLAUDE.md 明令禁止的混用入口。

// fillTypicalCaseDisplayNames 在 TypeName/RoleName/LevelName 为空时按 code 填展示名。
// 已有非空 *Name 不覆盖，便于调用方自定义文案。
func fillTypicalCaseDisplayNames(p *types.AddTypicalCasePayload) {
	if p == nil {
		return
	}
	if p.TypeName == "" {
		if n, ok := types.TypicalCaseTypeNames[p.Type]; ok {
			p.TypeName = n
		}
	}
	if p.RoleName == "" {
		if n, ok := types.TypicalCaseRoleNames[p.Role]; ok {
			p.RoleName = n
		}
	}
	if p.LevelName == "" {
		if n, ok := types.TypicalCaseLevelNames[p.Level]; ok {
			p.LevelName = n
		}
	}
}

// typicalCaseCodeString 把 type/role/level 统一成映射表用的字符串代码。
// 列表回填常见 number（int/float64），新增表单为 string；均需可识别。
func typicalCaseCodeString(v any) (string, bool) {
	switch n := v.(type) {
	case string:
		if n == "" {
			return "", false
		}
		return n, true
	case int:
		return strconv.Itoa(n), true
	case int64:
		return strconv.FormatInt(n, 10), true
	case float64:
		// JSON 数字默认 float64；仅接受整数值，避免 2.5 误映射
		if n != float64(int64(n)) {
			return "", false
		}
		return strconv.FormatInt(int64(n), 10), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return "", false
		}
		return strconv.FormatInt(i, 10), true
	default:
		return "", false
	}
}

// fillTypicalCaseDisplayNamesMap 更新路径：map 含 type/role/level 且对应 *Name 缺失时补全。
// code 支持 string 与 number（对齐 getTypicalCase 列表响应 + 手填 string）。
func fillTypicalCaseDisplayNamesMap(payload map[string]any) {
	if payload == nil {
		return
	}
	if typeName, _ := payload["typeName"].(string); typeName == "" {
		if code, ok := typicalCaseCodeString(payload["type"]); ok {
			if n, ok := types.TypicalCaseTypeNames[code]; ok {
				payload["typeName"] = n
			}
		}
	}
	if roleName, _ := payload["roleName"].(string); roleName == "" {
		if code, ok := typicalCaseCodeString(payload["role"]); ok {
			if n, ok := types.TypicalCaseRoleNames[code]; ok {
				payload["roleName"] = n
			}
		}
	}
	if levelName, _ := payload["levelName"].(string); levelName == "" {
		if code, ok := typicalCaseCodeString(payload["level"]); ok {
			if n, ok := types.TypicalCaseLevelNames[code]; ok {
				payload["levelName"] = n
			}
		}
	}
}

// 典型案例 remark/content 的字数上限，对齐前端 el-input maxlength=
// classiccanter.vue:124 maxlength="198"（备注）、:130 maxlength="1500"（正文）。
// 浏览器硬截断保证线上恒发 ≤上限；SDK 不静默截断也不放行超长原文——
// 显式拒绝（ErrInvalidPayload），与 task content 的 maxTaskContentRunes 纪律同族。
const (
	maxTypicalCaseRemarkRunes  = 198
	maxTypicalCaseContentRunes = 1500
)

// validateTypicalCaseLengths 校验典型案例 remark/content 的 rune 长度上限。
// 超长即返回 ErrInvalidPayload（调用方输入问题 → CLI 漏斗 400/exit3），不发业务请求。
func validateTypicalCaseLengths(payload *types.AddTypicalCasePayload) error {
	if payload == nil {
		return fmt.Errorf("%w: 典型案例 payload 为空", ErrInvalidPayload)
	}
	if len([]rune(payload.Remark)) > maxTypicalCaseRemarkRunes {
		return fmt.Errorf("%w: remark 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseRemarkRunes, len([]rune(payload.Remark)))
	}
	if len([]rune(payload.Content)) > maxTypicalCaseContentRunes {
		return fmt.Errorf("%w: content 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseContentRunes, len([]rune(payload.Content)))
	}
	return nil
}

// validateTypicalCaseLengthsMap 校验典型案例更新路径（map）的 remark/content
// rune 上限。与 validateTypicalCaseLengths（Add 路径）同族：超长即
// ErrInvalidPayload，不发业务请求。：此前 Update 无校验、Add 有，
// 长度纪律不对称。
func validateTypicalCaseLengthsMap(payload map[string]any) error {
	if len([]rune(firstStringFromMap(payload, "remark"))) > maxTypicalCaseRemarkRunes {
		return fmt.Errorf("%w: remark 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseRemarkRunes, len([]rune(firstStringFromMap(payload, "remark"))))
	}
	if len([]rune(firstStringFromMap(payload, "content"))) > maxTypicalCaseContentRunes {
		return fmt.Errorf("%w: content 超过 %d 字上限（收到 %d 字）",
			ErrInvalidPayload, maxTypicalCaseContentRunes, len([]rune(firstStringFromMap(payload, "content"))))
	}
	return nil
}

// firstStringFromMap 读取 map 中首个非空字符串键值（兼容 string/[]byte/数字）。
func firstStringFromMap(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

// AddTypicalCase 提交一条典型案例。
//
// 用户只需填标题/类别代码/角色代码/级别代码/指导教师等；
// TypeName/RoleName/LevelName 为空时按前端下拉自动补全。
// 遵循 AddHonor 模式：doBizVoid POST → 成功返回 nil。
func (c *Client) AddTypicalCase(ctx context.Context, token string, payload types.AddTypicalCasePayload) error {
	// ：remark/content 长度校验必须先于任何请求——前端 maxlength 截断保证
	// 线上恒发 ≤上限（198/1500 字），SDK 对超长原文显式拒绝（ErrInvalidPayload）。
	if err := validateTypicalCaseLengths(&payload); err != nil {
		return err
	}
	fillTypicalCaseDisplayNames(&payload)
	return c.doBizVoid(ctx, token, "AddTypicalCase",
		"/api/studentCircleNew/addTypicalCase", http.MethodPost, payload)
}

// TypicalCaseStatus 是典型案例审核状态，即 getTypicalCase 查询参数
// status 的线协议取值（与前端 classiccanter.vue 的下拉一致）。
//
// 与写实列表类型同理：status 直接驱动服务端的列表过滤，非法值不会报错，
// 只会静默返回一份意料之外的记录集合。做成具名类型并在发请求前 Valid，
// 是让它不再依赖「记得只在 0..3 里选」的根本手段。
//
// 它与写实状态（CircleRecord.Status：0 已发布 / 1 已锁定 / 2 被撤回）
// 不是同一概念，勿混读。
type TypicalCaseStatus int

const (
	// TypicalCaseStatusPending 未审核。
	TypicalCaseStatusPending TypicalCaseStatus = 0
	// TypicalCaseStatusApproved 审核通过。
	TypicalCaseStatusApproved TypicalCaseStatus = 1
	// TypicalCaseStatusRejected 审核驳回。
	TypicalCaseStatusRejected TypicalCaseStatus = 2
	// TypicalCaseStatusAll 全部（前端默认）。
	TypicalCaseStatusAll TypicalCaseStatus = 3
)

// Valid 报告该状态是否为平台承认的四种之一。
func (s TypicalCaseStatus) Valid() bool {
	switch s {
	case TypicalCaseStatusPending, TypicalCaseStatusApproved,
		TypicalCaseStatusRejected, TypicalCaseStatusAll:
		return true
	default:
		return false
	}
}

// typicalCaseStatusFromValue 从查询参数值反查审核状态，供 CLI 的 --status
// 解析与 SDK 共享同一份映射，避免两侧各写一张表。
func typicalCaseStatusFromValue(v int) (TypicalCaseStatus, error) {
	s := TypicalCaseStatus(v)
	if !s.Valid() {
		return 0, fmt.Errorf("%w: 非法典型案例审核状态 %d（合法值 0=未审核 1=通过 2=驳回 3=全部）",
			ErrInvalidPayload, v)
	}
	return s, nil
}

// resolveTypicalCaseStatus 收敛两个列表入口共用的状态判定：变参缺省时
// 取「全部」，给了值则先判合法性再放行。归 ErrInvalidPayload 使调用方
// 映射为 400 / 退出码 3，且不发出任何业务请求。
func resolveTypicalCaseStatus(status []int) (int, error) {
	if len(status) == 0 {
		return int(TypicalCaseStatusAll), nil
	}
	s, err := typicalCaseStatusFromValue(status[0])
	if err != nil {
		return 0, err
	}
	return int(s), nil
}

// typicalCaseListPath 拼装 getTypicalCase 查询串，是两个列表入口的
// 唯一路径来源。此前两处各自拼装同一串查询参数，结构化与透传路径
// 改一处就会与另一处脱节。
func typicalCaseListPath(pageNo, pageSize, status int) string {
	return "/api/studentCircleNew/getTypicalCase?pageNo=" + strconv.Itoa(pageNo) +
		"&pageSize=" + strconv.Itoa(pageSize) + "&status=" + strconv.Itoa(status)
}

// GetTypicalCaseList 查询典型案例列表（分页）。
//
// status 为可选变参：不传时默认 3（全部），与前端默认一致。
// 取值：0 未审 / 1 通过 / 2 驳回 / 3 全部；非法值在发请求前归
// ErrInvalidPayload（400 / 退出码 3），不发出任何业务请求。
//
// 签名：GetTypicalCaseList(ctx, token, pageNo, pageSize, status...int)
// 多传 status 时仅用第一个。
func (c *Client) GetTypicalCaseList(ctx context.Context, token string, pageNo, pageSize int, status ...int) (*types.TypicalCaseListResult, error) {
	st, err := resolveTypicalCaseStatus(status)
	if err != nil {
		return nil, err
	}
	path := typicalCaseListPath(pageNo, pageSize, st)

	resp, err := c.doBizAndDecode(ctx, token, "GetTypicalCaseList", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseList 失败: %w", err)
	}

	pb, err := types.DecodePageBean(*resp)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseList 解析分页信息失败: %w", err)
	}

	records, err := types.DecodeDataList[types.TypicalCaseRecord](*resp)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseList 解析记录失败: %w", err)
	}

	return &types.TypicalCaseListResult{Records: records, Page: pb}, nil
}

// GetTypicalCaseListJSON 返回典型案例列表的原始 JSON（CLI 1:1 对齐）。
//
// status 变参语义同 GetTypicalCaseList：默认 3（全部），非法值同样在
// 发请求前被拒。拼装 {"records":..., "page":...}，records 和 page 均为
// 平台原始字节。
func (c *Client) GetTypicalCaseListJSON(ctx context.Context, token string, pageNo, pageSize int, status ...int) (json.RawMessage, error) {
	st, err := resolveTypicalCaseStatus(status)
	if err != nil {
		return nil, err
	}
	path := typicalCaseListPath(pageNo, pageSize, st)

	resp, err := c.doBizAndDecode(ctx, token, "GetTypicalCaseListJSON", path, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("GetTypicalCaseListJSON 失败: %w", err)
	}
	return assembleRecordsPageJSON(resp), nil
}

// UpdateTypicalCase 更新一条典型案例。
// POST /api/studentCircleNew/updateTypicalCase
//
// 与 AddTypicalCase 对称：type/role/level 有值且对应 *Name 为空时自动补展示名。
// remark/content 长度校验与 Add 同族（198/1500 rune，ErrInvalidPayload）。
func (c *Client) UpdateTypicalCase(ctx context.Context, token string, payload map[string]any) error {
	if err := validateTypicalCaseLengthsMap(payload); err != nil {
		return err
	}
	fillTypicalCaseDisplayNamesMap(payload)
	return c.doBizVoid(ctx, token, "UpdateTypicalCase",
		"/api/studentCircleNew/updateTypicalCase", http.MethodPost, payload)
}

// DeleteTypicalCase 删除一条典型案例。
// GET /api/studentCircleNew/deleteTypicalCase?id=
func (c *Client) DeleteTypicalCase(ctx context.Context, token string, id int64) error {
	path := "/api/studentCircleNew/deleteTypicalCase?id=" + strconv.FormatInt(id, 10)
	return c.doBizVoid(ctx, token, "DeleteTypicalCase", path, http.MethodGet, nil)
}

// DeleteBatchTypicalCase 批量删除典型案例。
// POST /api/studentCircleNew/deleteBatchTypicalCase
//
// 请求体是纯 JSON 数组 [1, 2, 3]（前端源码确认）。
// 空/nil 切片返回 ErrInvalidPayload 而非发出字面 null 请求体
// （对齐前端 classiccanter.vue 的空数组守卫与 CLI 层参数校验口径）。
func (c *Client) DeleteBatchTypicalCase(ctx context.Context, token string, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: 批量删除需要至少一个 id", ErrInvalidPayload)
	}
	return c.doBizVoid(ctx, token, "DeleteBatchTypicalCase",
		"/api/studentCircleNew/deleteBatchTypicalCase", http.MethodPost, ids)
}
