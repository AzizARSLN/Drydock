package modules

import (
	"fmt"
	"strings"
)

// UnifiedDiff produces a compact line-based diff between two texts. It is used
// to preview pending changes during a dry-run before anything is applied.
func UnifiedDiff(path, before, after string) string {
	a := splitLines(before)
	b := splitLines(after)
	if len(a) == len(b) {
		same := true
		for i := range a {
			if a[i] != b[i] {
				same = false
				break
			}
		}
		if same {
			return ""
		}
	}

	// Guard against pathological inputs: fall back to a coarse summary.
	const maxLines = 4000
	if len(a) > maxLines || len(b) > maxLines {
		return fmt.Sprintf("--- %s\n+++ %s\n(cok buyuk dosya: %d -> %d satir)\n",
			path, path, len(a), len(b))
	}

	lcs := longestCommonSubsequence(a, b)
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)

	i, j, k := 0, 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case k < len(lcs) && i < len(a) && a[i] == lcs[k]:
			sb.WriteString("  " + a[i] + "\n")
			i++
			j++
			k++
		case k < len(lcs) && j < len(b) && b[j] == lcs[k]:
			sb.WriteString("  " + b[j] + "\n")
			i++
			j++
			k++
		case i < len(a) && (k >= len(lcs) || a[i] != lcs[k]):
			sb.WriteString("- " + a[i] + "\n")
			i++
		default:
			sb.WriteString("+ " + b[j] + "\n")
			j++
		}
	}
	return sb.String()
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// longestCommonSubsequence returns the LCS of two string slices.
func longestCommonSubsequence(a, b []string) []string {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, a[i])
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return out
}
