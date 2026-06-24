package schedule

import (
	"testing"
	"time"
)

func TestNewOffsetSchedule_空偏移_报错(t *testing.T) {
	base := SolarAnnual{Month: 3, Day: 5, Hour: 9}
	if _, err := NewOffsetSchedule(base, nil); err == nil {
		t.Error("空偏移应返回 error")
	}
}

func TestOffsetSchedule_NextAfter(t *testing.T) {
	at := func(y, m, d, h, min int) time.Time {
		return time.Date(y, time.Month(m), d, h, min, 0, 0, time.Local)
	}
	day3 := -3 * 24 * time.Hour // T-3d

	base := SolarAnnual{Month: 3, Day: 5, Hour: 9} // 每年 3-5 09:00

	tests := []struct {
		name    string
		offsets []time.Duration
		since   time.Time
		want    time.Time
	}{
		{
			name:    "今年T-3d还没到_返回今年03-02",
			offsets: []time.Duration{day3},
			since:   at(2027, 2, 1, 0, 0),
			want:    at(2027, 3, 2, 9, 0),
		},
		{
			// 关键临界：since 落在今年 T-3d(03-02) 之后、生日(03-05) 之前。
			// 今年的 T-3d 已过，必须滚到明年；符号写反会错误返回今年 03-02(早于 since)。
			name:    "临界_T3d已过生日未到_滚到明年03-02",
			offsets: []time.Duration{day3},
			since:   at(2027, 3, 4, 9, 0),
			want:    at(2028, 3, 2, 9, 0),
		},
		{
			name:    "多偏移_取最早的那条(T-3d早于T-0)",
			offsets: []time.Duration{0, day3}, // T-0 和 T-3d
			since:   at(2027, 1, 1, 0, 0),
			want:    at(2027, 3, 2, 9, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os, err := NewOffsetSchedule(base, tt.offsets)
			if err != nil {
				t.Fatalf("构造失败: %v", err)
			}
			got, ok := os.NextAfter(tt.since)
			if !ok {
				t.Fatalf("应有下次，得到 ok=false")
			}
			if !got.Equal(tt.want) {
				t.Errorf("since=%v\n期望 %v\n得到   %v", tt.since, tt.want, got)
			}
			if !got.After(tt.since) {
				t.Errorf("结果 %v 未严格晚于 since %v", got, tt.since)
			}
		})
	}
}

func TestOffsetSchedule_序列交替(t *testing.T) {
	at := func(y, m, d, h, min int) time.Time {
		return time.Date(y, time.Month(m), d, h, min, 0, 0, time.Local)
	}
	base := SolarAnnual{Month: 3, Day: 5, Hour: 9}
	os, _ := NewOffsetSchedule(base, []time.Duration{-3 * 24 * time.Hour, 0}) // T-3d, T-0

	// 从 2027-01-01 起连续推进，应交替产出 03-02、03-05、次年 03-02、03-05...
	want := []time.Time{
		at(2027, 3, 2, 9, 0),
		at(2027, 3, 5, 9, 0),
		at(2028, 3, 2, 9, 0),
		at(2028, 3, 5, 9, 0),
	}
	cur := at(2027, 1, 1, 0, 0)
	for i, w := range want {
		got, ok := os.NextAfter(cur)
		if !ok {
			t.Fatalf("第 %d 次应有下次", i)
		}
		if !got.Equal(w) {
			t.Fatalf("第 %d 次：期望 %v，得到 %v", i, w, got)
		}
		cur = got // 用上次触发时刻推进，模拟 engine
	}
}
