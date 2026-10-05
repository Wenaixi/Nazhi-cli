package main

import (
	"context"
	"fmt"

	"github.com/Wenaixi/Nazhi-cli/pkg/client"
	"github.com/Wenaixi/Nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleCmd 表示 nazhi circle 父命令
var circleCmd = &cobra.Command{
	Use:   "circle",
	Short: "写实管理",
	Long:  `管理写实记录：删除、评论、点赞。`,
}

// circleDeleteCmd 表示 nazhi circle delete 命令
var circleDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "删除一条写实记录",
	Long:    "删除指定 ID 的写实记录。删除后不可恢复。",
	Example: "  nazhi circle delete --id 123456 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 先校后建：--id 非法在建客户端之前就拒。缺 --token 与 --id 非法
		// 同时发生时用户先看到该修的那个错（与 honor list / typical-case list 同派）。
		id, ok := circleIDFromFlag(cmd)
		if !ok {
			return
		}
		runReadOp(cmd, readOpMode{
			verboseMsg:  fmt.Sprintf("正在删除写实记录 id=%d...", id),
			errorPrefix: "删除写实记录失败",
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				// DeleteCircle 成功路径无业务负载；fetch 必须回一个非 nil 值，
				// 否则 runner 的 normalizeEmptyList 会把 nil 归一成空数组，
				// success 闭包便无从区分「成功」与「无负载」。
				return emptyPayload{}, c.DeleteCircle(ctx, token, id)
			},
			success: func(any) *envelope.Envelope {
				// 成功但无业务负载 → envelope.Empty（HTTP 204），与 SDK 语义 1:1。
				return envelope.Empty("删除成功")
			},
		})
	},
}

// emptyPayload 是「调用成功但无业务负载」的占位载荷。
//
// 为什么需要它：fetch 的返回值会被 runner 的 normalizeEmptyList 过一遍，
// nil 会被归一为空数组，于是 success 闭包无法再用 result == nil 区分
// 「删除成功」与「评论成功但服务端未返回对象」。零尺寸类型的形状明确，
// 且不会与任何真实载荷类型混淆。
type emptyPayload struct{}

func init() {
	circleCmd.AddCommand(circleDeleteCmd)
	circleDeleteCmd.Flags().String("id", "", "写实记录 ID（必填）")
	registerBizFlags(circleDeleteCmd)
}
