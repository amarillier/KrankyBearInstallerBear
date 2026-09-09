package innoimport

import "strings"

// parseInnoAttrs splits one Inno [Files]-style directive line
// ("Source: "..."; DestDir: "..."; Flags: a b; Excludes: "...""") into its
// "Key: Value" attributes, keyed by Key with surrounding quotes stripped
// from Value. Splitting on ";" is quote-aware — Inno's own Excludes
// attribute may itself hold several ";"-separated patterns inside one
// quoted value, which a naive strings.Split(line, ";") would cut apart
// incorrectly.
func parseInnoAttrs(line string) map[string]string {
	attrs := make(map[string]string)
	for _, field := range splitOutsideQuotes(line, ';') {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		attrs[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return attrs
}

// splitOutsideQuotes splits s on sep, treating any sep byte that falls
// between a pair of double quotes as literal content rather than a
// separator.
func splitOutsideQuotes(s string, sep byte) []string {
	var fields []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
			cur.WriteByte(c)
		case c == sep && !inQuotes:
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	fields = append(fields, cur.String())
	return fields
}
