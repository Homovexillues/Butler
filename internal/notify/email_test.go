package notify

import "testing"

func TestNewEmailNotifier_参数校验(t *testing.T) {
	// 一组合法参数，各用例在它基础上改坏一个字段
	valid := func() (host string, port int, username, authcode, from string, to []string) {
		return "smtp.qq.com", 465, "me@qq.com", "authcode123", "me@qq.com", []string{"me@qq.com"}
	}

	t.Run("全部合法_不报错", func(t *testing.T) {
		h, p, u, a, f, to := valid()
		if _, err := NewEmailNotifier(h, p, u, a, f, to); err != nil {
			t.Errorf("合法参数不应报错，得到: %v", err)
		}
	})

	tests := []struct {
		name    string
		breakIt func(*string, *int, *string, *string, *string, *[]string)
	}{
		{"host为空", func(h *string, _ *int, _, _, _ *string, _ *[]string) { *h = "" }},
		{"port为0", func(_ *string, p *int, _, _, _ *string, _ *[]string) { *p = 0 }},
		{"username为空", func(_ *string, _ *int, u, _, _ *string, _ *[]string) { *u = "" }},
		{"authcode为空", func(_ *string, _ *int, _, a, _ *string, _ *[]string) { *a = "" }},
		{"from为空", func(_ *string, _ *int, _, _, f *string, _ *[]string) { *f = "" }},
		{"to为空", func(_ *string, _ *int, _, _, _ *string, to *[]string) { *to = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_应报错", func(t *testing.T) {
			h, p, u, a, f, to := valid()
			tt.breakIt(&h, &p, &u, &a, &f, &to)
			if _, err := NewEmailNotifier(h, p, u, a, f, to); err == nil {
				t.Errorf("%s 时应返回 error，却返回 nil", tt.name)
			}
		})
	}
}
