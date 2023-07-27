package access

const (
	apiKeyHeaderField = "X-Api-Key" // #nosec G101
	suffixLen         = 10
)

func keySuffixLogging(k string) string {
	var suffix string
	if n := len(k); n > suffixLen {
		suffix = k[n-suffixLen:]
	} else {
		suffix = k
	}

	return suffix
}
