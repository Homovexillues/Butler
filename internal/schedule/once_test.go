package schedule

import (
	"testing"
	"time"
)

func TestOnce_NextAfter(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.Local)

	t.Run("未来时刻_返回该时刻且ok", func(t *testing.T) {
		target := now.Add(time.Hour)
		o := Once{At: target}

		next, ok := o.NextAfter(now)
		if !ok {
			t.Fatalf("期望 ok=true，得到 false")
		}
		if !next.Equal(target) {
			t.Errorf("期望 next=%v，得到 %v", target, next)
		}
	})

	t.Run("过去时刻_返回ok为false", func(t *testing.T) {
		o := Once{At: now.Add(-time.Hour)}

		_, ok := o.NextAfter(now)
		if ok {
			t.Errorf("过去的一次性任务应返回 ok=false，却返回了 true")
		}
	})

	t.Run("恰好相等_不算严格晚于_返回false", func(t *testing.T) {
		o := Once{At: now}

		_, ok := o.NextAfter(now)
		if ok {
			t.Errorf("触发时刻等于 since 时不满足严格晚于，应返回 ok=false")
		}
	})
}
