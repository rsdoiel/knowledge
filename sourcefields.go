package knowledge

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

// doiShape is the form of a bare DOI: the "10." directory indicator, a
// registrant code of digits (optionally with dotted subdivisions), a slash and
// a non-empty suffix with no whitespace. It checks shape only. Real registrant
// codes have four or more digits, but this module's own fixtures use 10.1/a,
// so a longer minimum would refuse data the tests already treat as valid.
var doiShape = regexp.MustCompile(`^10\.[0-9]+(\.[0-9]+)*/\S+$`)

// cleanSource trims and checks the fields of a source about to be added, and
// returns the tidied copy. AddSource calls it; the JSONL importer does not, so
// an export from a database that already holds an odd value still restores.
//
// Every field is trimmed. The title must not be blank. published_date, when
// set, must be YYYY, YYYY-MM or YYYY-MM-DD and a real calendar date. A url
// identifier must parse as an absolute URL with a scheme and a host. A doi
// identifier must be a bare DOI: nothing is normalised, so a pasted
// https://doi.org/... form is refused rather than rewritten and the dedupe key
// stays exactly what the caller supplied (less padding). Other identifier types
// (isbn, issn, arxiv, urn) have no shape checked.
func cleanSource(s Source) (Source, error) {
	title, err := cleanSourceTitle(s.Title)
	if err != nil {
		return s, err
	}
	s.Title = title
	s.IdentifierValue = strings.TrimSpace(s.IdentifierValue)
	s.PublishedDate = strings.TrimSpace(s.PublishedDate)

	if s.PublishedDate != "" && !validPartialDate(s.PublishedDate) {
		return s, invalidf("knowledge: invalid published date %q; want YYYY, YYYY-MM or YYYY-MM-DD", s.PublishedDate)
	}
	switch s.IdentifierType {
	case "url":
		if !validAbsoluteURL(s.IdentifierValue) {
			return s, invalidf("knowledge: invalid url %q; want an absolute URL such as https://example.org/page", s.IdentifierValue)
		}
	case "doi":
		if !doiShape.MatchString(s.IdentifierValue) {
			return s, invalidf("knowledge: invalid doi %q; want the bare form 10.NNNN/suffix, without a doi: or https://doi.org/ prefix", s.IdentifierValue)
		}
	}
	return s, nil
}

// cleanSourceTitle trims a source title and refuses a blank one, the same rule
// CleanName applies to project and concept names.
func cleanSourceTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", invalidf("knowledge: source title must not be empty")
	}
	return title, nil
}

// validPartialDate reports whether s is YYYY, YYYY-MM or YYYY-MM-DD and names a
// real calendar date. Partial dates are legal because a citation often has
// only a year.
func validPartialDate(s string) bool {
	layout := map[int]string{4: "2006", 7: "2006-01", 10: "2006-01-02"}[len(s)]
	if layout == "" {
		return false
	}
	_, err := time.Parse(layout, s)
	return err == nil
}

// validAbsoluteURL reports whether s parses as a URL with both a scheme and a
// host and holds no whitespace. mailto: and file:///path have no host and are
// refused: a source's url identifier names a page on a server.
func validAbsoluteURL(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}
