package parser

import (
	"testing"
	"time"

	"butler/internal/action"
)

func TestPlanToNodes_LastFired绑定原始树节点(t *testing.T) {
	plan := PlanNode{
		Children: []PlanNode{
			{
				Title: "测试分组",
				Children: []PlanNode{
					{
						Title: "测试任务",
						Once:  "2099-01-01 00:00:00",
						NotifyAction: &action.NotifyAction{
							Channels: []string{"mqtt"},
						},
					},
				},
			},
		},
	}

	nodes, err := PlanToNodes(&plan)
	if err != nil {
		t.Fatalf("PlanToNodes() 返回错误：%v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("拍平后节点数量 = %d，期望 1", len(nodes))
	}

	sourceLastFired := &plan.Children[0].Children[0].LastFired
	if nodes[0].LastFired != sourceLastFired {
		t.Fatal("运行时 Node.LastFired 没有指向原始 PlanNode.LastFired")
	}

	firedAt := time.Date(2026, 8, 26, 15, 30, 0, 0, time.Local)
	*nodes[0].LastFired = firedAt

	if !plan.Children[0].Children[0].LastFired.Equal(firedAt) {
		t.Fatalf(
			"原始 PlanNode.LastFired = %s，期望 %s",
			plan.Children[0].Children[0].LastFired,
			firedAt,
		)
	}
}
