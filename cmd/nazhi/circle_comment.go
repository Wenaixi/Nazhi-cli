package main

import (
	"context"
	"errors"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleCommentCmd 表示 nazhi circle comment 命令
var circleCommentCmd = &cobra.Command{
	Use:     "comment",
	Short:   "添加写实评论",
	Long:    "给指定写实记录添加评论。",
	Example: "  nazhi circle comment --id 123456 --content '写得好' --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// 两条校验的次序是既有契约：先 --id 后 --content。
		// 两者都在建客户端之前，与 circle delete / like 同派。
		id, ok := circleIDFromFlag(cmd)
		if !ok {
			return
		}
		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			printParamError(errors.New("--content 为必填"))
			return
		}
		runReadOp(cmd, readOpMode{
			verboseMsg:  "正在添加评论...",
			errorPrefix: "添加评论失败",
			fetch: func(ctx context.Context, c *client.Client, token string) (any, error) {
				// AddCircleComment 成功时 returnData 缺失会返回 (nil, nil)：
				// 那不是错误（前端 commentList.unshift 拿到什么就 unshift 什么），
				// 归一成占位载荷，由 success 闭包走 Empty 分支。
				comment, err := c.AddCircleComment(ctx, token, id, content)
				if err != nil {
					return nil, err
				}
				if comment == nil {
					return emptyPayload{}, nil
				}
				return comment, nil
			},
			success: func(result any) *envelope.Envelope {
				if _, empty := result.(emptyPayload); empty {
					return envelope.Empty("评论成功")
				}
				return envelope.Success(result)
			},
		})
	},
}

func init() {
	circleCmd.AddCommand(circleCommentCmd)
	circleCommentCmd.Flags().String("id", "", "写实记录 ID（必填）")
	circleCommentCmd.Flags().String("content", "", "评论内容（必填）")
	registerBizFlags(circleCommentCmd)
}
