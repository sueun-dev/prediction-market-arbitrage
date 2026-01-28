package market

import (
	"fmt"
	"math/big"
	"strings"
)

const WeiDecimals = 18

// FormatDecimal formats a Wei-style integer string to a decimal string.
// value is a base-10 integer string, decimals is the number of fractional digits
// in the integer representation, precision is the max fractional digits to show.
func FormatDecimal(value string, decimals, precision int) string {
	if value == "" {
		value = "0"
	}

	negative := false
	raw := value
	if strings.HasPrefix(value, "-") {
		negative = true
		raw = value[1:]
	}

	num := new(big.Int)
	num.SetString(raw, 10)
	if num == nil {
		num = big.NewInt(0)
	}

	base := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(num, base)
	fracPart := new(big.Int).Mod(num, base)

	fracStr := fracPart.String()
	if len(fracStr) < decimals {
		fracStr = strings.Repeat("0", decimals-len(fracStr)) + fracStr
	}

	frac := fracStr
	if precision < len(frac) {
		frac = frac[:precision]
	}
	frac = strings.TrimRight(frac, "0")

	var out string
	if len(frac) > 0 {
		out = fmt.Sprintf("%s.%s", intPart.String(), frac)
	} else {
		out = intPart.String()
	}

	if negative {
		return "-" + out
	}
	return out
}

// FormatPercent formats a Wei value as a percentage string.
func FormatPercent(value string, decimals, precision int) string {
	adjusted := decimals - 2
	if adjusted < 0 {
		adjusted = 0
	}
	return FormatDecimal(value, adjusted, precision) + "%"
}
