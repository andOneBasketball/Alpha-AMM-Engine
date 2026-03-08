package handler

import "github.com/shopspring/decimal"

func DecimalSqrt(x decimal.Decimal) decimal.Decimal {
	if x.IsZero() {
		return decimal.Zero
	}

	z := x
	prev := decimal.Zero
	two := decimal.NewFromInt(2)

	for i := 0; i < 1000; i++ {

		z = z.Add(x.Div(z)).Div(two)

		if z.Sub(prev).Abs().LessThan(decimal.NewFromFloat(1e-18)) {
			break
		}

		prev = z
	}

	return z
}
