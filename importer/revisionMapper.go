package importer

import (
	"regexp"

	"github.com/stevejefferson/trac2gitea/log"
)

var (
	revisionRangeRegexp = regexp.MustCompile(`(^|[^\w]\s*)(r\d+)-(?:r)?(\d+)`)
	revisionRegexp      = regexp.MustCompile(`(^|[^\w]\s*)(r\d+)`)
	changesetRegexp     = regexp.MustCompile(`In (?:\[\d+\])?changeset:"?(\d+)"?:`)
	bracketLinkRegexp   = regexp.MustCompile(`\[(\d+)\]`)
)

func mapRevision(in string, revisionMap map[string]string) string {
	return revisionRegexp.ReplaceAllStringFunc(in, func(match string) string {
		svnRef := revisionRegexp.ReplaceAllString(match, `$2`)
		if gitRef, found := revisionMap[svnRef]; found == true && gitRef != "" {
			return revisionRegexp.ReplaceAllString(match, `$1`) + gitRef
		} else {
			log.Warn("No matching commit for svn revision '%s'", svnRef)
		}
		return match
	})
}

func mapRevisionRange(in string, revisionMap map[string]string) string {
	return revisionRangeRegexp.ReplaceAllStringFunc(in, func(match string) string {
		svnRefStart := revisionRangeRegexp.ReplaceAllString(match, `$2`)
		svnRefEnd := "r" + revisionRangeRegexp.ReplaceAllString(match, `$3`)

		if gitRefStart, found := revisionMap[svnRefStart]; found == true && gitRefStart != "" {
			if gitRefEnd, found := revisionMap[svnRefEnd]; found == true && gitRefEnd != "" {
				return revisionRangeRegexp.ReplaceAllString(match, `$1`) + gitRefStart + ".." + gitRefEnd
			} else {
				log.Warn("No matching commit for svn revision '%s'", svnRefEnd)
			}
		} else {
			log.Warn("No matching commit for svn revision '%s'", svnRefStart)
		}

		return match
	})
}

func mapChangeset(in string, revisionMap map[string]string) string {
	return changesetRegexp.ReplaceAllStringFunc(in, func(match string) string {
		svnRef := "r" + changesetRegexp.ReplaceAllString(match, `$1`)
		if gitRef, found := revisionMap[svnRef]; found == true && gitRef != "" {
			// NOTE: No colon since Gitea won't link it then.
			return "See " + gitRef
		} else {
			log.Warn("No matching commit for svn revision '%s'", svnRef)
		}
		return match
	})
}

func mapBracketLink(in string, revisionMap map[string]string) string {
	return bracketLinkRegexp.ReplaceAllStringFunc(in, func(match string) string {
		svnRef := bracketLinkRegexp.ReplaceAllString(match, `r$1`)
		if gitRef, found := revisionMap[svnRef]; found == true && gitRef != "" {
			return gitRef
		} else {
			log.Warn("No matching commit for svn revision '%s'", svnRef)
		}
		return match
	})
}

func MapRevisions(in string, revisionMap map[string]string) string {
	if revisionMap == nil {
		return in
	}

	out := mapRevisionRange(in, revisionMap)
	out = mapRevision(out, revisionMap)
	out = mapChangeset(out, revisionMap)
	out = mapBracketLink(out, revisionMap) // Must be done after mapChangeset which may include a bracketted number

	return out
}
