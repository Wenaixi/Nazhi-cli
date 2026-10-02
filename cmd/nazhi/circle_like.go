package main

import (
	"context"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleLikeCmd 表示 nazhi circle like 命令
var circleLikeCmd = &cobra.Command{
	Use:     "like",
	Short:   "点赞/取消点赞写实记录",
	Long:    "给指定 ID 的写实记录点赞或取消点赞。服务端自动切换点赞/取消状态。",
	Example: "  nazhi circle like --id 123456 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 先校后建：--id 非法在建客户端之前就拒（与 circle delete / comment 同派）。
		id, ok := circleIDFromFlag(cmd)
		if !ok {
			return
		}
		runReadOp(cmd, readOpMode{
			verboseMsg:  "正在点赞...",
			errorPrefix: "点赞失败",
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				// SetCircleLike 成功路径无业务负载，返回占位载荷理由同 circle delete。
				return emptyPayload{}, c.SetCircleLike(ctx, token, id)
			},
			success: func(any) *envelope.Envelope { return envelope.Empty("操作成功") },
		})
	},
}

func init() {
	circleCmd.AddCommand(circleLikeCmd)
	circleLikeCmd.Flags().String("id", "", "写实记录 ID（必填）")
	registerBizFlags(circleLikeCmd)
}
