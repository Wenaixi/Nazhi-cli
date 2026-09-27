package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// typicalCaseCmd 表示 nazhi typical-case 父命令
//
//	nazhi typical-case submit --token <token> --payload '<json>'
//	nazhi typical-case list --token <token> [--page <页>] [--page-size <条>]
var typicalCaseCmd = &cobra.Command{
	Use:   "typical-case",
	Short: "典型案例管理",
	Long:  `管理综合评价典型案例：提交典型案例、查看已提交记录。`,
}

// typicalCaseSubmitCmd 表示 nazhi typical-case submit 命令
//
//	nazhi typical-case submit --token <token> --payload '<json>'
//	成功时 envelope.Empty("典型案例提交成功")，与 AddHonor 写操作模式一致
var typicalCaseSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交典型案例",
	Long: `提交一条典型案例。payload 是 addTypicalCase 请求体 JSON，
可用 @file.json 从文件读取，或 - 从 stdin 读取。`,
	Example: `  nazhi typical-case submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"title":"...","type":"1","teacherName":"王隆滨","partnerName":"合作者","remark":"任务描述","content":"..."}'
  nazhi typical-case submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @case.json
  echo '{"title":"..."}' | nazhi typical-case submit --token "xxx" --payload -`,
	Args: cobra.NoArgs, // 输入全走 flag，位置参数无语义；与 delete-batch 及全仓多数派对齐
	Run: func(cmd *cobra.Command, args []string) {
		runWriteOp(cmd, typicalCaseSubmitWriteOp, nil)
	},
}

// typicalCaseListCmd 表示 nazhi typical-case list 命令
//
//	nazhi typical-case list --token <token> [--page <页>] [--page-size <条>] [--status <状态>]
//	envelope.data 透传 SDK GetTypicalCaseListJSON 的原始 JSON（含 records + page）
//	status：0 未审 / 1 通过 / 2 驳回 / 3 全部（默认 3，与前端一致）
var typicalCaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取典型案例列表",
	Long: `获取当前用户的典型案例记录（分页）。
--status 筛选审核状态：0 未审核 / 1 通过 / 2 驳回 / 3 全部（默认）。`,
	Example: `  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx --status 1
  nazhi typical-case list --token eyJhbGciOiJIUzI1NiJ9.xxx --page 1 --page-size 10`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 先校后建——分页参数校验必须在 buildBizClient 前，
		// 否则缺 token + 坏分页参数时首报「--token 必填」而非分页错误
		// （honor list 同款收敛，对齐 output.go 披露的「先校后建」派）。
		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		status, _ := cmd.Flags().GetInt("status")
		// 分页纪律由 validatePaginationFlags 单点持有（honor list 同款）：
		// 正整数守卫与 maxPageSize 上钳的错误文案由常量派生。
		// status 合法值为 0/1/2/3（0 未审核 / 1 通过 / 2 驳回 / 3 全部·默认，
		// 前端 classiccanter.vue el-option 相同）。判定由 SDK 侧的
		// TypicalCaseStatus.Valid 在发请求前完成，归 400 / 退出码 3。
		//
		// 此前此处刻意不校验，理由是「避免破坏可能用 -1 表达全部的用户脚本」。
		// 该理由无据：全仓、文档与前端下拉均无 -1 这一取值，脚本若真依赖它，
		// 早已被服务端当作未知状态处理。审核状态是驱动列表过滤的参数，
		// 放行越界值只会静默返回意料之外的记录集合。
		if err := validatePaginationFlags(pageNo, pageSize); err != nil {
			printParamError(err)
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在获取典型案例列表...")
		raw, err := c.GetTypicalCaseListJSON(cmd.Context(), token, pageNo, pageSize, status)
		if err != nil {
			printError(fmt.Errorf("获取典型案例列表失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	// typical-case submit
	typicalCaseCmd.AddCommand(typicalCaseSubmitCmd)
	typicalCaseSubmitCmd.Flags().String("payload", "", "典型案例 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(typicalCaseSubmitCmd)
	attachAllowedKeysHelp(typicalCaseSubmitCmd, typicalCaseSubmitKeys.display(), false)

	// typical-case list
	typicalCaseCmd.AddCommand(typicalCaseListCmd)
	typicalCaseListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	typicalCaseListCmd.Flags().Int("page-size", 10, "每页条数")
	typicalCaseListCmd.Flags().Int("status", 3, "审核状态：0 未审 / 1 通过 / 2 驳回 / 3 全部（默认）")
	registerBizFlags(typicalCaseListCmd)

	// typical-case update
	typicalCaseCmd.AddCommand(typicalCaseUpdateCmd)
	typicalCaseUpdateCmd.Flags().String("payload", "", "典型案例 JSON（必填，可用 @file.json）")
	registerBizFlags(typicalCaseUpdateCmd)
	attachAllowedKeysHelp(typicalCaseUpdateCmd, typicalCaseUpdateKeys.display(), false)

	// typical-case delete
	typicalCaseCmd.AddCommand(typicalCaseDeleteCmd)
	typicalCaseDeleteCmd.Flags().String("id", "", "案例 ID（必填）")
	registerBizFlags(typicalCaseDeleteCmd)

	// typical-case delete-batch
	typicalCaseCmd.AddCommand(typicalCaseDeleteBatchCmd)
	typicalCaseDeleteBatchCmd.Flags().String("payload", "", "典型案例 ID 数组（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(typicalCaseDeleteBatchCmd)
}

// typicalCaseDeleteBatchCmd 表示 nazhi typical-case delete-batch 命令。
var typicalCaseDeleteBatchCmd = &cobra.Command{
	Use:     "delete-batch",
	Short:   "批量删除典型案例",
	Long:    "批量删除典型案例。payload 必须是正整数 ID 数组，可用 @file.json 或 - 读取。",
	Example: "  nazhi typical-case delete-batch --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '[1,2,3]'",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printParamError(errors.New("--payload 为必填"))
			return
		}
		ids, err := parseTypicalCaseBatchIDs(cmd.Context(), payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		printVerbose("正在批量删除典型案例...")
		if err := c.DeleteBatchTypicalCase(cmd.Context(), token, ids); err != nil {
			printError(fmt.Errorf("批量删除典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例已批量删除"))
	},
}

// parseTypicalCaseBatchIDs 读取并校验批量删除的纯 ID 数组。
func parseTypicalCaseBatchIDs(ctx context.Context, raw string) ([]int64, error) {
	payload, err := parsePayloadFromArg(ctx, raw)
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := json.Unmarshal(payload, &ids); err != nil {
		return nil, fmt.Errorf("顶层 JSON 必须是正整数数组: %w", err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("批量 ID 数组不能为空")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("批量 ID 必须为正整数，实际 %d", id)
		}
	}
	return ids, nil
}

// typicalCaseUpdateKeys 是 typical-case update payload 顶层 JSON 的允许键，
// 按用户书写形态声明：AddTypicalCasePayload 出站 json 键 +
// SDK UpdateTypicalCase 消费键 + 编辑记录 id。
// 未知键（如 titel 拼错）此前静默透传服务端被忽略，对齐 user update 拒绝。
var typicalCaseUpdateKeys = newPayloadKeySet(
	"id", "title", "type", "typeName", "teacherName", "partnerName",
	"role", "roleName", "remark", "content", "level", "levelName",
	"attachmentId", "attachmentName",
)

// typicalCaseSubmitKeys 是 typical-case submit payload 顶层 JSON 的允许键
// （AddTypicalCasePayload 出站 json 键全集）。submit 此前按 struct
// 反序列化静默丢弃未知顶层键，拼错键名（如 titlee）服务端忽略零字段——
// 与 update 同款拒绝（400/exit3）。
var typicalCaseSubmitKeys = newPayloadKeySet(
	"title", "type", "typeName", "teacherName", "partnerName",
	"role", "roleName", "remark", "content", "level", "levelName",
	"attachmentId", "attachmentName",
)

// 比较用小写允许集由 keySet 派生，不手工维护第二份。
var (
	typicalCaseUpdateAllowedKeys = typicalCaseUpdateKeys.allowed()
	typicalCaseAddAllowedKeys    = typicalCaseSubmitKeys.allowed()
)

// typicalCaseUpdateCmd 表示 nazhi typical-case update 命令。
var typicalCaseUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "更新典型案例",
	Long:  `按 ID 更新典型案例内容。payload 为 updateTypicalCase 请求体对象，必填 id 字段。`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runWriteOp(cmd, typicalCaseUpdateWriteOp, nil)
	},
}

// typicalCaseDeleteCmd 表示 nazhi typical-case delete 命令
var typicalCaseDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除典型案例",
	Long:  `按 ID 删除一条典型案例记录。--id 必填且必须为正整数。`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		idStr, _ := cmd.Flags().GetString("id")
		if idStr == "" {
			printParamError(errors.New("--id 为必填"))
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			printParamError(errors.New("--id 必须为正整数"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在删除典型案例...")
		if err := c.DeleteTypicalCase(cmd.Context(), token, id); err != nil {
			printError(fmt.Errorf("删除典型案例失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("典型案例已删除"))
	},
}
