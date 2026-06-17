package schedule

import (
	"testing"
	"time"
)

func TestNewCronSchedule_非法表达式_返回error(t *testing.T) {
	if _, err := NewCronSchedule("不是cron"); err == nil {
		t.Error("非法表达式应返回 error，却返回 nil")
	}
}

func TestCronSchedule_NextAfter(t *testing.T) {
	at := func(y, m, d, h, min int) time.Time {
		return time.Date(y, time.Month(m), d, h, min, 0, 0, time.Local)
	}

	tests := []struct {
		name  string
		spec  string
		since time.Time
		want  time.Time
	}{
		{
			name:  "每天9点_当天9点已过_返回明天9点",
			spec:  "0 9 * * *",
			since: at(2026, 6, 17, 10, 0),
			want:  at(2026, 6, 18, 9, 0),
		},
		{
			name:  "每天9点_当天9点未到_返回今天9点",
			spec:  "0 9 * * *",
			since: at(2026, 6, 17, 8, 0),
			want:  at(2026, 6, 17, 9, 0),
		},
		{
			name:  "每月1号9点_跨月",
			spec:  "0 9 1 * *",
			since: at(2026, 6, 17, 0, 0),
			want:  at(2026, 7, 1, 9, 0),
		},
		{
			name:  "整点触发_严格晚于since",
			spec:  "0 * * * *", // 每小时整点
			since: at(2026, 6, 17, 10, 0),
			want:  at(2026, 6, 17, 11, 0), // 不返回与 since 相等的 10:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCronSchedule(tt.spec)
			if err != nil {
				t.Fatalf("解析 %q 失败: %v", tt.spec, err)
			}
			got, ok := c.NextAfter(tt.since)
			if !ok {
				t.Fatalf("期望有下次，得到 ok=false")
			}
			if !got.Equal(tt.want) {
				t.Errorf("spec=%q since=%v\n期望 %v\n得到   %v", tt.spec, tt.since, tt.want, got)
			}
		})
	}
}
