package main

import (
	"context"
	"errors"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskDimensionsCmd 获取平台写实维度列表。
var taskDimensionsCmd = &cobra.Command{
	Use:     "dimensions",
	Short:   "获取写实维度列表",
	Long:    "获取目标平台的写实维度列表，供后续类别和任务查询使用。",
	Example: "  nazhi task dimensions --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runReadOp(cmd, readOpMode{
			verboseMsg:  "正在获取写实维度...",
			errorPrefix: "获取写实维度失败",
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				return c.GetDimensions(ctx, token)
			},
			success: readListSuccess,
		})
	},
}

// taskCircleTypeCmd 获取任务提交所需的 circleTypeId、dimensionId、hours 等元数据。
var taskCircleTypeCmd = &cobra.Command{
	Use:     "circle-type",
	Short:   "获取任务写实元数据",
	Long:    "按任务 ID 获取提交写实记录所需的类别、维度、学时等平台元数据。",
	Example: "  nazhi task circle-type --token eyJhbGciOiJIUzI1NiJ9.xxx --task-id 18154",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runReadOp(cmd, readOpMode{
			verboseMsg:  "正在获取任务写实元数据...",
			errorPrefix: "获取任务写实元数据失败",
			validate: func(cmd *cobra.Command) error {
				taskID, _ := cmd.Flags().GetInt64("task-id")
				if taskID <= 0 {
					return errors.New("--task-id 必须为正整数")
				}
				return nil
			},
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				taskID, _ := cmd.Flags().GetInt64("task-id")
				return c.GetCircleTypeByTaskID(ctx, token, taskID)
			},
			// 空元数据 → Empty envelope（无任务写实元数据），不是空数组。
			success: func(result any) *envelope.Envelope {
				if result == nil {
					return envelope.Empty("未找到任务写实元数据")
				}
				return envelope.Success(result)
			},
		})
	},
}

func init() {
	taskCmd.AddCommand(taskDimensionsCmd)
	registerBizFlags(taskDimensionsCmd)

	taskCmd.AddCommand(taskCircleTypeCmd)
	taskCircleTypeCmd.Flags().Int64("task-id", 0, "平台任务 ID（必填）")
	registerBizFlags(taskCircleTypeCmd)
}
