package schedule

import (
	"testing"
	"time"
)

func TestSolarAnnual_NextAfter(t *testing.T) {
	// at 用本地时区造时间，让用例可读
	at := func(y, m, d, h, min int) time.Time {
		return time.Date(y, time.Month(m), d, h, min, 0, 0, time.Local)
	}

	tests := []struct {
		name  string
		sched SolarAnnual
		since time.Time
		want  time.Time
	}{
		{
			name:  "今年的还没到_返回今年",
			sched: SolarAnnual{Month: 12, Day: 25, Hour: 9},
			since: at(2026, 6, 17, 0, 0),
			want:  at(2026, 12, 25, 9, 0),
		},
		{
			name:  "今年的已过_滚到明年",
			sched: SolarAnnual{Month: 3, Day: 5, Hour: 9},
			since: at(2026, 6, 17, 0, 0),
			want:  at(2027, 3, 5, 9, 0),
		},
		{
			name:  "恰好相等_严格晚于_滚到明年",
			sched: SolarAnnual{Month: 6, Day: 17, Hour: 0},
			since: at(2026, 6, 17, 0, 0),
			want:  at(2027, 6, 17, 0, 0),
		},
		{
			name:  "2月29_目标年为平年_回退到2月28",
			sched: SolarAnnual{Month: 2, Day: 29, Hour: 9},
			since: at(2026, 6, 17, 0, 0), // 下一个落点 2027，平年
			want:  at(2027, 2, 28, 9, 0),
		},
		{
			name:  "2月29_目标年为闰年_保持2月29",
			sched: SolarAnnual{Month: 2, Day: 29, Hour: 9},
			since: at(2027, 6, 17, 0, 0), // 下一个落点 2028，闰年
			want:  at(2028, 2, 29, 9, 0),
		},
		{
			name:  "2月29当年是闰年但当天已过_滚到下一年并回退",
			sched: SolarAnnual{Month: 2, Day: 29, Hour: 9},
			since: at(2028, 3, 1, 0, 0), // 2028 闰年但 2/29 已过 → 2029 平年 → 回退 2/28
			want:  at(2029, 2, 28, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.sched.NextAfter(tt.since)
			if !ok {
				t.Fatalf("SolarAnnual 应永远有下次，得到 ok=false")
			}
			if !got.Equal(tt.want) {
				t.Errorf("since=%v\n期望 %v\n得到   %v", tt.since, tt.want, got)
			}
			// 额外保证：结果必须严格晚于 since
			if !got.After(tt.since) {
				t.Errorf("结果 %v 未严格晚于 since %v", got, tt.since)
			}
		})
	}
}
