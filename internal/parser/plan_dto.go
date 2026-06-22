package parser

type Plan struct {
	Children []PlanNode
}

type PlanNode struct {
	Title        string
	Body         string
	Once         string
	Solar        string
	Lunar        string
	Cron         string
	NotifyOffset []string
	Channels     []string
	Children     []PlanNode
}
