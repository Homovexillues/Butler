package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"butler/internal/config"
	"butler/internal/model"
	"butler/internal/parser"

	"github.com/spf13/cobra"
	"golang.org/x/text/width"
)

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "展示数据库中的所有任务及下次计划时间",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := config.LoadDatabase()
		if err != nil {
			return fmt.Errorf("load database: %w", err)
		}
		defer s.Close()
		tasks, err := s.GetAll(cmd.Context())
		if err != nil {
			return fmt.Errorf("load tasks: %w", err)
		}
		return printTasks(cmd.OutOrStdout(), tasks, time.Now())
	},
}

func printTasks(out io.Writer, tasks []model.Task, now time.Time) error {
	if len(tasks) == 0 {
		_, err := fmt.Fprintln(out, "暂无任务。")
		return err
	}
	rows := [][]string{{"ID", "标题", "状态", "调度", "渠道", "下次计划时间"}}
	type plannedRow struct {
		cells []string
		next  time.Time
		found bool
	}
	planned := make([]plannedRow, 0, len(tasks))
	for _, task := range tasks {
		state, next := "禁用", "无后续触发"
		var nextAt time.Time
		var found bool
		if task.Enabled {
			state = "启用"
		}
		channels := make([]string, len(task.Channels))
		for i, channel := range task.Channels {
			channels[i] = channel.String()
		}
		schedule, err := parser.MakeSchedule(task.ScheduleType, task.ScheduleExpression, task.ScheduleOffset)
		if err != nil {
			next = "配置错误：" + err.Error()
		} else if at, ok := schedule.NextAfter(now); ok {
			nextAt, found = at, true
			next = at.In(time.Local).Format("2006-01-02 15:04:05")
		}

		planned = append(planned, plannedRow{
			cells: []string{fmt.Sprint(task.ID), task.Title, state, string(task.ScheduleType), strings.Join(channels, ","), next},
			next:  nextAt, found: found,
		})
	}
	sort.SliceStable(planned, func(i, j int) bool {
		if planned[i].found != planned[j].found {
			return planned[i].found
		}
		return planned[i].found && planned[i].next.Before(planned[j].next)
	})
	for _, row := range planned {
		rows = append(rows, row.cells)
	}
	sizes := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			row[i] = strings.Join(strings.Fields(cell), " ")
			sizes[i] = max(sizes[i], terminalWidth(row[i]))
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			padding := ""
			if i < len(row)-1 {
				padding = strings.Repeat(" ", sizes[i]-terminalWidth(cell)+2)
			}
			if _, err := fmt.Fprint(out, cell, padding); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	return nil
}

// Count CJK characters as two columns so Chinese task titles stay aligned.
func terminalWidth(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			n += 2
		default:
			n++
		}
	}
	return n
}

func init() { rootCmd.AddCommand(treeCmd) }
