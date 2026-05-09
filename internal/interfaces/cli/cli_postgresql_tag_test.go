//go:build postgresql

package cli

func cliMetadataValueEqual(a, b any) bool {
	aFloat, aIsNum := cliToFloat64(a)
	bFloat, bIsNum := cliToFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	return a == b
}

func cliToFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
