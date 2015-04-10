package util

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var level = 0

// func debugf(f string, vs ...interface{}) {
// 	log.Printf(strings.Repeat("  ", level)+f, vs...)
// }

// ShortRegexpString tries to construct a short regexp that matches exactly the
// provided strings and nothing else.
func ShortRegexpString(vs ...string) (res string) {
	switch len(vs) {
	case 0:
		return "$.^" // Unmatchable?
	case 1:
		return regexp.QuoteMeta(vs[0]) // Nothing else to do.
	}

	// level++
	// defer func(s string) {
	// 	level--
	// 	debugf("ShortRegexpString(%s) = %#q", s, res)
	// }(fmt.Sprintf("%#q", vs))

	recurse := func(prefix, suffix string, data commonSs) (result string) {
		//debugf("> recurse(%#q, %#q, %v) on %#q", prefix, suffix, data, vs)
		if data.start > 0 {
			result += ShortRegexpString(dup(vs[:data.start])...) + "|"
		}

		//debugf("%v/%#q/%#q: %v\n", vs, prefix, suffix, data)
		varying := make([]string, data.end-data.start)
		for i := data.start; i < data.end; i++ {
			varying[i-data.start] = vs[i][len(prefix) : len(vs[i])-len(suffix)]
		}
		middle := ShortRegexpString(varying...)
		//debugf(">> ShortRegexpString(%#q) = %#q", varying, middle)
		opt := ""
		if strings.HasPrefix(middle, "|") {
			middle = middle[1:]
			opt = "?"
		}
		if len(middle) > 1 {
			middle = fmt.Sprintf("(%s)%s", middle, opt)
		} else if middle != "" {
			middle += opt
		}
		result += prefix + middle + suffix

		if data.end < len(vs) {
			result += "|" + ShortRegexpString(dup(vs[data.end:])...)
		}
		return result
	}

	// The length of a naive solution: N strings plus N-1 separators.
	bestCost := -1
	for _, v := range vs {
		bestCost += len(regexp.QuoteMeta(v)) + 1
	}
	best := ""

	found := false

	sort.Sort(reverseStrings(vs))
	vs = removeDups(vs)
	// debugf("Reverse-sorted: %#q", vs)
	for suffix, sufLoc := range commonSuffixes(vs, 2) {
		// sufLoc := suffixes[suffix]
		prefix := sharedPrefix(len(suffix), vs[sufLoc.start:sufLoc.end])
		str := recurse(prefix, suffix, sufLoc)
		if len(str) < bestCost || (len(str) == bestCost && str < best) {
			bestCost = len(str)
			best = str
			found = true
		} else {
			//debugf("! rejected %#q", str)
			//debugf("  because: %#q", best)
		}
	}

	sort.Strings(vs)
	// debugf("Sorted: %#q", vs)
	for prefix, preLoc := range commonPrefixes(vs, 2) {
		suffix := sharedSuffix(len(prefix), vs[preLoc.start:preLoc.end])
		str := recurse(prefix, suffix, preLoc)
		if len(str) < bestCost || (len(str) == bestCost && str < best) {
			bestCost = len(str)
			best = str
			found = true
		} else {
			//debugf("! rejected %#q", str)
			//debugf("  because: %#q", best)
		}
	}

	if found {
		return best
	}

	// Last resort: Just put ORs between them (after escaping meta-characters)
	for i := range vs {
		vs[i] = regexp.QuoteMeta(vs[i])
	}
	return strings.Join(vs, "|")
}

// removeDups removes duplicate strings from vs and returns it.
// It assumes that vs has been sorted such that duplicates are next to each
// other.
func removeDups(vs []string) []string {
	insertPos := 1
	for i := 1; i < len(vs); i++ {
		if vs[i-1] != vs[i] {
			vs[insertPos] = vs[i]
			insertPos++
		}
	}
	return vs[:insertPos]
}

func dup(vs []string) []string {
	result := make([]string, len(vs))
	copy(result, vs)
	return result
}

// keys returns a sorted array of keys.
func keys(m map[string]commonSs) (result []string) {
	result = make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// commonSs holds information on where to find a common substring.
type commonSs struct {
	start, end int
}

// commonPrefixes returns a map from prefixes to number of occurrences. Not all
// strings in vs need to have a prefix for it to be returned.
// Assumes vs to have been sorted with sort.Strings()
func commonPrefixes(vs []string, minLength int) (result map[string]commonSs) {
	result = make(map[string]commonSs)
	for i := 0; i < len(vs)-1; i++ {
		j := i + 1
		k := 0
		for ; k < len(vs[i]) && k < len(vs[j]); k++ {
			if vs[i][k] != vs[j][k] {
				break
			}
		}
		if k < minLength {
			continue
		}
		prefix := vs[i][:k]
		if _, exists := result[prefix]; !exists {
			first := prefixStart(vs[:i], prefix)
			//debugf("prefixStart(%#q, %#q) == %v", vs[:i], prefix, first)
			// prefixEnd(vs, prefix) - first + 1
			// == prefixEnd(vs[first:], prefix) + 1
			// == prefixEnd(vs[first+1:], prefix) + 2
			end := first + 1 + prefixEnd(vs[first+1:], prefix)
			result[prefix] = commonSs{
				first, end,
			}
			//debugf("prefixEnd(%#q, %#q) == %v", vs, prefix, result[prefix].end)
		}
	}
	// debugf("# %v..", result)
	return result
}

func prefixStart(vs []string, prefix string) int {
	if prefix == "" {
		return 0
	}
	return findFirst(vs, func(s string) bool {
		return strings.HasPrefix(s, prefix)
	})
}

func prefixEnd(vs []string, prefix string) int {
	if prefix == "" {
		return len(vs)
	}
	//debugf("prefixEnd(%v, %#q)", vs, prefix)
	return findFirst(vs, func(s string) bool {
		return !strings.HasPrefix(s, prefix)
	})
}

// reverseStrings is a sort.Interface that sort strings by their reverse values.
type reverseStrings []string

func (rs reverseStrings) Less(i, j int) bool {
	for m, n := len(rs[i])-1, len(rs[j])-1; m >= 0 && n >= 0; m, n = m-1, n-1 {
		if rs[i][m] != rs[j][n] {
			// We want to compare runes, not bytes. So find the start of the
			// current runes and decode them.
			for ; m > 0 && !utf8.RuneStart(rs[i][m]); m-- {
			}
			for ; n > 0 && !utf8.RuneStart(rs[j][n]); n-- {
			}
			ri, _ := utf8.DecodeRuneInString(rs[i][m:])
			rj, _ := utf8.DecodeRuneInString(rs[j][n:])
			return ri < rj
		}
	}
	return len(rs[i]) < len(rs[j])
}
func (rs reverseStrings) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }
func (rs reverseStrings) Len() int      { return len(rs) }

// commonSuffixes returns a map from suffixes to number of occurrences. Not all
// strings in vs need to have a suffix for it to be returned.
// Assumes vs to have been sorted using sort.Sort(reverseStrings(vs))
func commonSuffixes(vs []string, minLength int) (result map[string]commonSs) {
	result = make(map[string]commonSs)
	for i := 0; i < len(vs)-1; i++ {
		j := i + 1
		k := 0
		for ; k < len(vs[i]) && k < len(vs[j]); k++ {
			if vs[i][len(vs[i])-k-1] != vs[j][len(vs[j])-k-1] {
				break
			}
		}
		if k < minLength {
			continue
		}
		suffix := vs[i][len(vs[i])-k:]
		if _, exists := result[suffix]; !exists {
			first := suffixStart(vs[:i], suffix)
			//debugf("suffixStart<%#q>(%#q) == %v", suffix, vs[:i], first)
			// suffixEnd(vs, suffix) - first + 1
			// == suffixEnd(vs[first:], suffix) + 1
			// == suffixEnd(vs[first+1:], suffix) + 2
			end := first + 1 + suffixEnd(vs[first+1:], suffix)
			result[suffix] = commonSs{
				first, end,
			}
			//debugf("suffixEnd  <%#q>(%#q) == %v", suffix, vs, result[suffix].end)
			//debugf("selected(%#q): %q\n\n", suffix, vs[first:result[suffix].end])
		}
	}
	// debugf("# ..%v", result)
	return result
}

func suffixStart(vs []string, suffix string) int {
	// //debugf("suffixStart(%#q, %#q)", vs, suffix)
	if suffix == "" {
		return 0
	}
	return findFirst(vs, func(s string) bool {
		return strings.HasSuffix(s, suffix)
	})
}

func suffixEnd(vs []string, suffix string) int {
	// //debugf("suffixEnd  (%#q, %#q)", vs, suffix)
	if suffix == "" {
		return len(vs)
	}
	return findFirst(vs, func(s string) bool {
		return !strings.HasSuffix(s, suffix)
	})
}

// findFirst finds the first element of vs that satisfies the predicate.
// It assumes that the first N strings don't match the predicate, and the rest
// do. If all of the strings satisfy the predicate, it returns 0, and if none
// do it returns len(vs).
func findFirst(vs []string, predicate func(string) bool) int {
	l, h := -1, len(vs)
	// Invariant: vs[l] does not match, vs[h] does.
	// -1 and len(vs) are sentinal values, never tested but assumed to mismatch and match, respectively.
	for l+1 < h {
		m := (l + h) / 2 // Must now be a valid value
		// //debugf("%d %d %d", l, m, h)
		if predicate(vs[m]) {
			h = m
		} else {
			l = m
		}
	}
	//debugf("==> %d", h)
	return h
}

// sharedPrefix returns the longest prefix which all the parameters share but
// ignores a number of characters at the end of each string.
func sharedPrefix(ignore int, vs []string) (result string) {
	//debugf("sharedPrefix(%d, %#q)", ignore, vs)
	// defer func() {
	//debugf("==> %#q", result)
	// }()
	switch len(vs) {
	case 0:
		return ""
	case 1:
		return vs[0]
	}
	for i := 0; i < len(vs[0])-ignore; i++ {
		for n := 1; n < len(vs); n++ {
			if i >= len(vs[n])-ignore || vs[0][i] != vs[n][i] {
				return vs[0][:i]
			}
		}
	}
	return vs[0][:len(vs[0])-ignore]
}

// sharedSuffix returns the longest suffix which all the parameters share but
// ignores a number of characters at the start of each string.
func sharedSuffix(ignore int, vs []string) (result string) {
	//debugf("sharedSuffix(%d, %#q)", ignore, vs)
	// defer func() {
	//debugf("==> %#q", result)
	// }()
	switch len(vs) {
	case 0:
		return ""
	case 1:
		return vs[0]
	}
	for i := 0; i < len(vs[0])-ignore; i++ {
		for n := 1; n < len(vs); n++ {
			if i >= len(vs[n])-ignore || vs[0][len(vs[0])-i-1] != vs[n][len(vs[n])-i-1] {
				return vs[0][len(vs[0])-i:]
			}
		}
	}
	return vs[0][ignore:]
}
