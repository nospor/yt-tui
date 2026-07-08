package ytcli

import "testing"

func TestNormalizeAssignee(t *testing.T) {
	client := &Client{}

	tests := []struct {
		input    string
		expected string
	}{
		{"Przemek Słupecki", "przemek.slupecki"},
		{"François", "francois"},
		{"Müller", "muller"},
		{"Renée", "renee"},
		{"John Doe", "john.doe"},
		{"unassigned", ""},
		{"-", ""},
		{"   ", ""},
		{"ą ć ę ł ń ó ś ź ż", "a.c.e.l.n.o.s.z.z"},
		{"Ą Ć Ę Ł Ń Ó Ś Ź Ż", "a.c.e.l.n.o.s.z.z"},
		{"ø Ø æ Æ ß ð Ð đ Đ þ Þ", "o.o.ae.ae.ss.d.d.d.d.th.th"},
	}

	for _, tt := range tests {
		actual := client.normalizeAssignee(tt.input)
		if actual != tt.expected {
			t.Errorf("normalizeAssignee(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}
