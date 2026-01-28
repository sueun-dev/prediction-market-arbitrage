package market

import "testing"

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		value     string
		decimals  int
		precision int
		want      string
	}{
		{"0", 18, 6, "0"},
		{"1000000000000000000", 18, 6, "1"},
		{"1500000000000000000", 18, 6, "1.5"},
		{"1234567890000000000", 18, 6, "1.234567"},
		{"1234567890123456789", 18, 6, "1.234567"},
		{"100000000000000000", 18, 6, "0.1"},
		{"10000000000000000", 18, 6, "0.01"},
		{"1000000000000000", 18, 6, "0.001"},
		{"-1500000000000000000", 18, 6, "-1.5"},
		{"", 18, 6, "0"},
		{"0", 18, 2, "0"},
		{"1234000000000000000", 18, 2, "1.23"},
		{"50000000000000000", 18, 6, "0.05"},
		// Smaller decimals
		{"12345", 4, 2, "1.23"},
		{"100", 2, 2, "1"},
	}

	for _, tt := range tests {
		got := FormatDecimal(tt.value, tt.decimals, tt.precision)
		if got != tt.want {
			t.Errorf("FormatDecimal(%q, %d, %d) = %q, want %q",
				tt.value, tt.decimals, tt.precision, got, tt.want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		value     string
		decimals  int
		precision int
		want      string
	}{
		{"500000000000000000", 18, 2, "50%"},
		{"1000000000000000000", 18, 2, "100%"},
		{"0", 18, 2, "0%"},
		{"123456789000000000", 18, 2, "12.34%"},
	}

	for _, tt := range tests {
		got := FormatPercent(tt.value, tt.decimals, tt.precision)
		if got != tt.want {
			t.Errorf("FormatPercent(%q, %d, %d) = %q, want %q",
				tt.value, tt.decimals, tt.precision, got, tt.want)
		}
	}
}
