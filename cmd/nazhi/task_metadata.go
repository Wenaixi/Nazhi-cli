package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wenaixi/Nazhi-cli/pkg/client"
	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
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

// taskCategoriesCmd 获取指定维度下的写实类别（支持 categories / types）。
var taskCategoriesCmd = &cobra.Command{
	Use:     "categories",
	Aliases: []string{"types"},
	Short:   "获取写实类别",
	Long:    "按维度获取写实类别。pid 可选，用于透传平台类别树的父节点。",
	Example: "  nazhi task categories --token eyJhbGciOiJIUzI1NiJ9.xxx --dimension-id 14 --pid 0",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		dimensionID, _ := cmd.Flags().GetInt64("dimension-id")
		pid, _ := cmd.Flags().GetString("pid")
		runReadOp(cmd, readOpMode{
			verboseMsg:  fmt.Sprintf("正在获取写实类别 dimensionId=%d...", dimensionID),
			errorPrefix: "获取写实类别失败",
			validate: func(*cobra.Command) error {
				if dimensionID <= 0 {
					return fmt.Errorf("--dimension-id 必须为正整数")
				}
				return nil
			},
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				return c.GetTaskCategories(ctx, token, dimensionID, pid)
			},
			success: readListSuccess,
		})
	},
}

// taskItemsCmd 获取指定类别下的写实任务（支持 items / tasks）。
var taskItemsCmd = &cobra.Command{
	Use:     "items",
	Aliases: []string{"tasks"},
	Short:   "获取类别下的写实任务",
	Long:    "按写实类别 ID 获取可用任务及其平台字段。",
	Example: "  nazhi task items --token eyJhbGciOiJIUzI1NiJ9.xxx --type-id 9274",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		typeID, _ := cmd.Flags().GetInt64("type-id")
		runReadOp(cmd, readOpMode{
			verboseMsg:  fmt.Sprintf("正在获取类别下写实任务 typeId=%d...", typeID),
			errorPrefix: "获取类别下写实任务失败",
			validate: func(*cobra.Command) error {
				if typeID <= 0 {
					return fmt.Errorf("--type-id 必须为正整数")
				}
				return nil
			},
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				return c.GetTaskItems(ctx, token, typeID)
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

// taskRecentCmd 获取学生当前最近填报的写实任务。
var taskRecentCmd = &cobra.Command{
	Use:     "recent",
	Short:   "获取最近提交的写实任务",
	Long:    "获取学生当前最近填报的写实任务列表（对齐前端学生主页 getRecentlyCircleTask）。",
	Example: "  nazhi task recent --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runReadOp(cmd, readOpMode{
			verboseMsg:  "正在获取最近提交任务...",
			errorPrefix: "获取最近提交任务失败",
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				return c.GetRecentlyCircleTask(ctx, token)
			},
			success: readListSuccess,
		})
	},
}

func init() {
	taskCmd.AddCommand(taskDimensionsCmd)
	registerBizFlags(taskDimensionsCmd)

	taskCmd.AddCommand(taskCategoriesCmd)
	taskCategoriesCmd.Flags().Int64("dimension-id", 0, "写实维度 ID（必填）")
	taskCategoriesCmd.Flags().String("pid", "", "类别树父节点 ID（可选）")
	registerBizFlags(taskCategoriesCmd)

	taskCmd.AddCommand(taskItemsCmd)
	taskItemsCmd.Flags().Int64("type-id", 0, "写实类别 ID（必填）")
	registerBizFlags(taskItemsCmd)

	taskCmd.AddCommand(taskCircleTypeCmd)
	taskCircleTypeCmd.Flags().Int64("task-id", 0, "平台任务 ID（必填）")
	registerBizFlags(taskCircleTypeCmd)

	taskCmd.AddCommand(taskRecentCmd)
	registerBizFlags(taskRecentCmd)
}
