package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wenaixi/Nazhi-cli/pkg/client"
	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskListCmd 表示 nazhi task list 命令
//
//	nazhi task list --token <token> [--base-url <url>] [--timeout <秒>]
//
// CLI 透传 SDK FetchTasks 的最终业务模型输出。
// SDK 在进入 CLI 前就已完成字段语义整理：
//   - circleTaskStatus → submitted
//   - upPic → needPic
//   - 日期字段为 string 透传（服务端原始格式，如 "2026-01-12"）
var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取全维度任务列表",
	Long:  `拉取目标平台全部维度的任务列表。内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。`,
	Example: `  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在获取任务列表...")
		tasks, err := c.FetchTasks(cmd.Context(), token)
		if err != nil {
			// partial 只在「确有部分结果」时成立，故两个纯前置失败的哨兵
			// （ErrEmptyUserInfo / ErrSessionBackoff）不列入：它们只由 session
			// 预热产生，而预热失败时 FetchTasks 走首个出口返回 nil 结果，
			// 恒不满足 len(tasks)>0 这半个合取项。判定清单因此只保留
			// 「维度级失败」的三类：业务拒绝与 context 取消/超时。
			isPartialErr := errors.Is(err, client.ErrBusinessRejected) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded)
			if isPartialErr && len(tasks) > 0 {
				printEnvelope(envelope.PartialData("fetch_tasks_partial_failure: "+err.Error(), tasks))
				return
			}
			printError(fmt.Errorf("获取任务列表失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(tasks))
	},
}

func init() {
	registerBizFlags(taskListCmd)
}
