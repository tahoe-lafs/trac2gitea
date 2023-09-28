// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package markdown

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/stevejefferson/trac2gitea/accessor/gitea"
	"github.com/stevejefferson/trac2gitea/accessor/trac"
	"github.com/stevejefferson/trac2gitea/log"
)

var (
	// regexp for trac '[[Image(<image>...,link=<link>)]]': $1=image, $2=link
	doubleBracketImageLinkRegexp = regexp.MustCompile(`\[\[Image\(([^,)]+)(?:, *link=([[:alnum:]\-._~:/?#@!$&'"(*+;%=]+))?[^\]]*\]\]`)

	// regexp for trac '[[<link>]]' and '[[<link>|<text>]]': $1=link, $2=text
	doubleBracketLinkRegexp = regexp.MustCompile(`\[\[([[:alpha:]][^|\]]*)(?:\|([^\]]+))?\]\]`)

	// regexp for trac '[<link>]' and '[<link> <text>]': $1=link, $2=text
	// note: trac image and double bracket links are processed before this so we do not need to exclude a leading '[' in the regexp
	singleBracketLinkRegexp = regexp.MustCompile(`\[([[:alpha:]][^ \]]*)(?: +([^\]]+))?\]`)

	// regexp for 'http://...' and 'https://...' links
	httpLinkRegexp = regexp.MustCompile(`https?://[[:alnum:]\-._~:/?#@!$&'"()*+,;%=]*[[:alnum:]]`)

	// regexp for trac 'htdocs:<link>': $1=link
	htdocsLinkRegexp = regexp.MustCompile(`htdocs:([[:alnum:]\-._~:/?#@!$&'"()*+,;%=]+)`)

	// regexp for local filenames used in Images (not explicit enough to be handled as attachment: or htdocs: links)
	// must match exactly
	localFileLinkRegexp = regexp.MustCompile(`^[[:alnum:]-._,]+\.[[:alpha:]]+$`)

	// regexp for a trac 'comment:<commentNum>' and 'comment:<commentNum>:ticket:<ticketID>' link: $1=commentNum, $2=ticketID
	ticketCommentLinkRegexp = regexp.MustCompile(`comment:([[:digit:]]+)(?::ticket:([[:digit:]]+))?`)

	// regexp for a trac 'milestone:<milestoneName>' link: $1=milestoneName
	milestoneLinkRegexp = regexp.MustCompile(`milestone:([[:alnum:]\-._~:/?#@!$&'"()*+,;%=]+)`)

	// regexp for a trac 'attachment:<file>', 'attachment:<file>:wiki:<pageName>' and 'attachment:<file>:ticket:<ticketID>' links: $1=file, $2=pageName, $3=ticketID
	attachmentLinkRegexp = regexp.MustCompile(
		`attachment:([[:alnum:]\-._~/?#@!$&'"()*+,;%=]+)` +
			`(?:` +
			`(?::wiki:((?:[[:upper:]][[:lower:]]*)+))|` +
			`(?::ticket:([[:digit:]]+))` +
			`)?`)

	// regexp for a trac 'changeset:<changesetID>' link: $1=commitID
	changesetLinkRegexp = regexp.MustCompile(`changeset:"([[:xdigit:]]+)/[^"]+"`)

	// regexp for a trac 'source:<sourcePath>' link: $1=sourcePath
	sourceLinkRegexp = regexp.MustCompile(`source:"[^/]+/([^"]+)"`)

	// regexp for a trac 'ticket:<ticketID>' link: $1=ticketID
	ticketLinkRegexp = regexp.MustCompile(`ticket:([[:digit:]]+)`)

	// regexp for trac 'wiki:<page>#<anchor>' links: $1=page $2=anchor
	// note: page does not need to be in proper CamelCase in this variant, but its last character should be alphanumeric
	wikiLinkRegexp = regexp.MustCompile(`wiki:([[:alnum:]:\-._&']*[[:alnum:]])(?:#([[:alnum:]?/:@\-._\~!$&'*+,;=]+))?`)

	// regexp for trac '<CamelCase>#anchor' wiki links: $1=leading char, $2=CamelCase $3=anchor
	// note: leading char (if any) must be a space or ']'
	//       - a space constitutes a "start of word" for an "standalone", unbracketted CamelCase link,
	//       - a zero-width space \u200B might be marking a removed opening square bracket from trac syntax
	//       - a ']' constitutes the end of the link comment after conversion of the various trac bracketting syntaxes above
	wikiCamelCaseLinkRegexp = regexp.MustCompile(`([[:space:]\x{200B}\]]|\A)((?:[[:upper:]][[:lower:]]+){2,})(?:#([[:alnum:]?/:@\-._\~!$&'()*+,;=]+))?`)

	// regexp for recognising a "marked" link with no accompanying text: $1=leading char, $2=link
	noTextMarkedLinkRegexp = regexp.MustCompile(`([^\]])\(@@([^@]+)@@\)`)

	// regexp for recognising a "marked" link: $1=link
	markedLinkRegexp = regexp.MustCompile(`\(@@([^@]+)@@\)`)

	// zero-width space for separating debracketted links from trailing text, until they are converted to proper markdown
	zeroWidthSpace = "\u200B"
)

// Link resolution functions:

//	These are responsible for extracting link information from its appropriate Trac link regexp and preparing that link for conversion to markdown.
//	The portion of the returned text corresponding to the link itself (as opposed to any extraneous characters that may have been hoovered up by the regexp)
//	should be "marked" using the markLink() function to identify it for later processing.

func (converter *DefaultConverter) resolveHTTPLink(link string) string {
	return markLink(link)
}

func (converter *DefaultConverter) resolveHtdocsLink(link string) string {
	// any htdocs file needs copying from trac htdocs directory to an equivalent wiki subdirectory
	htdocPath := htdocsLinkRegexp.ReplaceAllString(link, `$1`)
	tracHtdocPath := converter.tracAccessor.GetFullPath("htdocs", htdocPath)
	wikiHtdocRelPath := converter.giteaAccessor.GetWikiHtdocRelPath(htdocPath)
	converter.giteaAccessor.CopyFileToWiki(tracHtdocPath, wikiHtdocRelPath)
	wikiHtdocURL := converter.giteaAccessor.GetWikiFileURL(wikiHtdocRelPath)
	return markLink(wikiHtdocURL)
}

func (converter *DefaultConverter) resolveTicketCommentLink(ticketID int64, link string) string {
	commentNumStr := ticketCommentLinkRegexp.ReplaceAllString(link, `$1`)
	var commentNum int64
	commentNum, err := strconv.ParseInt(commentNumStr, 10, 64)
	if err != nil {
		log.Warn("found invalid Trac ticket comment number %s", commentNum)
		return link
	}

	commentTicketIDStr := ticketCommentLinkRegexp.ReplaceAllString(link, `$2`)
	var commentTicketID int64
	if commentTicketIDStr != "" {
		commentTicketID, err = strconv.ParseInt(commentTicketIDStr, 10, 64)
		if err != nil {
			log.Warn("found invalid Trac ticket id %s", commentTicketIDStr)
			return link
		}
	} else {
		// comment on current ticket
		if ticketID == trac.NullID {
			log.Warn("found Trac reference to comment %d of unknown ticket", commentNum)
			return link
		}
		commentTicketID = ticketID
	}

	issueID, err := converter.giteaAccessor.GetIssueID(commentTicketID)
	if err != nil {
		return link // not a recognised link - do not mark (error should already be logged)
	}
	if issueID == gitea.NullID {
		log.Warn("cannot find Gitea issue for ticket %d referenced by Trac link \"%s\"", commentTicketID, link)
		return link // not a recognised link - do not mark
	}

	// find gitea ID for trac comment
	timestamp, err := converter.tracAccessor.GetTicketCommentTime(commentTicketID, commentNum)
	if err != nil || timestamp == int64(0) {
		return link // not a recognised link - do not mark (error should already be logged)
	}
	commentIDs, err := converter.giteaAccessor.GetIssueCommentIDsByTime(issueID, timestamp)
	if err != nil || len(commentIDs) == 0 {
		return link // not a recognised link - do not mark (error should already be logged)
	}

	commentURL := converter.giteaAccessor.GetIssueCommentURL(issueID, commentIDs[0])
	return markLink(commentURL)
}

func (converter *DefaultConverter) resolveMilestoneLink(link string) string {
	milestoneName := milestoneLinkRegexp.ReplaceAllString(link, `$1`)
	milestoneID, err := converter.giteaAccessor.GetMilestoneID(milestoneName)
	if err != nil {
		return link // not a recognised link - do not mark (error should already be logged)
	}
	if milestoneID == gitea.NullID {
		log.Warn("cannot find milestone \"%s\" referenced by Trac link \"%s\"", milestoneName, link)
		return link // not a recognised link - do not mark
	}

	milestoneURL := converter.giteaAccessor.GetMilestoneURL(milestoneID)
	return markLink(milestoneURL)
}

func (converter *DefaultConverter) resolveTicketAttachmentLink(ticketID int64, attachmentName string, link string) string {
	issueID, err := converter.giteaAccessor.GetIssueID(ticketID)
	if err != nil {
		return link // not a recognised link - do not mark
	}
	if issueID == gitea.NullID {
		log.Warn("cannot find Gitea issue for ticket %d for Trac link \"%s\"", ticketID, link)
		return link // not a recognised link - do not mark
	}

	uuid, err := converter.giteaAccessor.GetIssueAttachmentUUID(issueID, attachmentName)
	if err != nil {
		return link // not a recognised link - do not mark
	}
	if uuid == "" {
		log.Warn("cannot find attachment \"%s\" for issue %d for Trac link \"%s\"", attachmentName, issueID, link)
		return link // not a recognised link - do not mark
	}

	attachmentURL := converter.giteaAccessor.GetIssueAttachmentURL(issueID, uuid)
	return markLink(attachmentURL)
}

func (converter *DefaultConverter) resolveWikiAttachmentLink(wikiPage string, attachmentName string, link string) string {
	attachmentWikiRelPath := converter.giteaAccessor.GetWikiAttachmentRelPath(wikiPage, attachmentName)
	attachmentURL := converter.giteaAccessor.GetWikiFileURL(attachmentWikiRelPath)
	return markLink(attachmentURL)
}

func (converter *DefaultConverter) resolveAttachmentLink(ticketID int64, wikiPage string, link string) string {
	attachmentName := attachmentLinkRegexp.ReplaceAllString(link, `$1`)
	attachmentWikiPage := attachmentLinkRegexp.ReplaceAllString(link, `$2`)
	attachmentTicketIDStr := attachmentLinkRegexp.ReplaceAllString(link, `$3`)

	// there are two types of attachment: ticket attachments and wiki attachments...
	if attachmentTicketIDStr != "" {
		var attachmentTicketID int64
		attachmentTicketID, err := strconv.ParseInt(attachmentTicketIDStr, 10, 64)
		if err != nil {
			log.Warn("found invalid Trac ticket id %s", attachmentTicketIDStr)
			return link
		}

		return converter.resolveTicketAttachmentLink(attachmentTicketID, attachmentName, link)
	} else if attachmentWikiPage != "" {
		return converter.resolveWikiAttachmentLink(attachmentWikiPage, attachmentName, link)
	}

	// no explicit ticket or wiki provided for attachment - use whichever of `ticketID` and `wiki` has been provided
	if ticketID != trac.NullID {
		return converter.resolveTicketAttachmentLink(ticketID, attachmentName, link)
	} else if wikiPage != "" {
		return converter.resolveWikiAttachmentLink(wikiPage, attachmentName, link)
	}

	log.Warn("Trac attachment link \"%s\" requires either ticket or wiki", link)
	return link
}

func (converter *DefaultConverter) resolveChangesetLink(link string) string {
	changesetID := changesetLinkRegexp.ReplaceAllString(link, `$1`)
	changesetURL := converter.giteaAccessor.GetCommitURL(changesetID)
	return markLink(changesetURL)
}

func (converter *DefaultConverter) resolveSourceLink(link string) string {
	sourcePath := sourceLinkRegexp.ReplaceAllString(link, `$1`)
	sourceURL := converter.giteaAccessor.GetSourceURL("master", sourcePath) // AFAICT Trac source URL does not include the git branch so we'll assume "master"
	return markLink(sourceURL)
}

func (converter *DefaultConverter) resolveTicketLink(link string) string {
	ticketIDStr := ticketLinkRegexp.ReplaceAllString(link, `$1`)
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil {
		log.Warn("found invalid Trac ticket reference %s" + link)
		return link // not a recognised link - do not mark
	}

	// validate ticket id
	issueID, err := converter.giteaAccessor.GetIssueID(ticketID)
	if err != nil {
		return link // not a recognised link - do not mark (error already logged)
	}
	if issueID == gitea.NullID {
		log.Warn("cannot find Gitea issue for ticket %d referenced by Trac link \"%s\"", ticketID, link)
		return link // not a recognised link - do not mark
	}

	issueURL := converter.giteaAccessor.GetIssueURL(issueID)
	return markLink(issueURL)
}

func (converter *DefaultConverter) resolveWikiLink(link string) string {
	wikiPageName := wikiLinkRegexp.ReplaceAllString(link, `$1`)
	wikiPageAnchor := wikiLinkRegexp.ReplaceAllString(link, `$2`)
	translatedPageName := converter.giteaAccessor.TranslateWikiPageName(wikiPageName)
	if wikiPageAnchor == "" {
		return markLink(translatedPageName)
	}
	return markLink(translatedPageName + "#" + wikiPageAnchor)
}

func (converter *DefaultConverter) resolveWikiCamelCaseLink(link string) string {
	leadingChar := wikiCamelCaseLinkRegexp.ReplaceAllString(link, `$1`)
	wikiPageName := wikiCamelCaseLinkRegexp.ReplaceAllString(link, `$2`)
	wikiPageAnchor := wikiCamelCaseLinkRegexp.ReplaceAllString(link, `$3`)
	translatedPageName := converter.giteaAccessor.TranslateWikiPageName(wikiPageName)
	if wikiPageAnchor == "" {
		return leadingChar + markLink(translatedPageName)
	}
	return leadingChar + markLink(translatedPageName+"#"+wikiPageAnchor)
}

// convertBrackettedTracLinks converts the various forms of (square) bracketted Trac links into an unbracketted form.
// The conversion performed here is partial: this method is solely responsible for disposing of the Trac bracketting
// - any resolution of actual trac links is done later
func (converter *DefaultConverter) convertBrackettedTracLinks(wikiPage string, in string) string {
	out := in

	out = doubleBracketImageLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		// convert Trac image links to markdown but leave the link unprocessed
		// - it will get dealt with later
		image := doubleBracketImageLinkRegexp.ReplaceAllString(match, "$1")
		link := doubleBracketImageLinkRegexp.ReplaceAllString(match, "$2")

		// If the image is just a local filename, we have to get its path, and mark that as a processed link
		if localFileLinkRegexp.MatchString(image) {
			image = markLink(converter.giteaAccessor.GetWikiAttachmentRelPath(wikiPage, image))
		}

		if link == "" {
			return "![]" + image + zeroWidthSpace
		}
		return "[![]" + image + "]" + link + zeroWidthSpace
	})

	out = doubleBracketLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		// convert Trac double bracket links into Trac single bracket links
		// - if we convert directly to markdown here, the "[<text>]" part of the markdown will get misinterpreted as a Trac single bracket link
		link := doubleBracketLinkRegexp.ReplaceAllString(match, "$1")
		text := doubleBracketLinkRegexp.ReplaceAllString(match, "$2")

		if text == "" {
			// '[[br]]' is a page break in Trac and is dealt with elsewhere
			if strings.EqualFold(link, "br") {
				return match
			}
			return "[" + link + "]"
		}

		return "[" + link + " " + text + "]"
	})

	out = singleBracketLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		// convert Trac single bracket links to markdown but leave the link unprocessed; it will get dealt with later
		link := singleBracketLinkRegexp.ReplaceAllString(match, "$1")
		text := singleBracketLinkRegexp.ReplaceAllString(match, "$2")

		if text == "" {
			// '[br]' can be assumed to be the inner section of a '[[br]]'
			if strings.EqualFold(link, "br") {
				return match
			}
			return zeroWidthSpace + link + zeroWidthSpace
		}

		return "[" + text + "]" + link + zeroWidthSpace
	})

	return out
}

// convertUnbrackettedTracLinks converts Trac-style links after any surrounding Trac bracketting and link texts have been processed
func (converter *DefaultConverter) convertUnbrackettedTracLinks(ticketID int64, wikiPage string, in string) string {
	out := in

	out = httpLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveHTTPLink(match)
	})

	out = htdocsLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveHtdocsLink(match)
	})

	out = ticketCommentLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveTicketCommentLink(ticketID, match)
	})

	out = milestoneLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveMilestoneLink(match)
	})

	out = attachmentLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveAttachmentLink(ticketID, wikiPage, match)
	})

	out = changesetLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveChangesetLink(match)
	})

	out = sourceLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveSourceLink(match)
	})

	// trac 'ticket:<ticketID>' and 'wiki:<pageName>' links can form suffixes to other trac links like attachments
	// so only process a standalone ticket of wiki link after we have handled the suffix cases above
	out = ticketLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveTicketLink(match)
	})

	out = wikiLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveWikiLink(match)
	})

	out = wikiCamelCaseLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		return converter.resolveWikiCamelCaseLink(match)
	})

	return out
}

// markLink marks the URL part of our links with a unique marker so that they can be picked up by later converters.
// We cannot just convert to markdown at this stage because markdown's round brackets are insufficiently unique
// and would result in the later converters misidentifying normal bracketted text as links.
// Hence we put a marker in here and later convert that marker to the final markdown.
func markLink(in string) string {
	return "(@@" + in + "@@)"
}

// unmarkLinks removes the "marking" placed around links by markLinks and converts them into their final markdown format
// after that, zero-width spaces can be removed
func (converter *DefaultConverter) unmarkLinks(in string) string {
	out := in
	out = noTextMarkedLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		// first replace (marked) links with no accompanying comment
		leadingChar := noTextMarkedLinkRegexp.ReplaceAllString(match, `$1`)
		markdownURL := noTextMarkedLinkRegexp.ReplaceAllString(match, `$2`)

		// if the link is a plain http(s) link, use a markdown automatic link
		if httpLinkRegexp.MatchString(markdownURL) {
			return leadingChar + "<" + markdownURL + ">"
		}

		// else, it is necessary to use a regular link, using the URL as accompanying comment
		return leadingChar + "[" + markdownURL + "]" + "(" + markdownURL + ")"
	})

	out = markedLinkRegexp.ReplaceAllStringFunc(out, func(match string) string {
		// any remaining (marked) links must have an accompanying comment so are converted into normal markdown links
		markdownURL := markedLinkRegexp.ReplaceAllString(match, `$1`)
		return "(" + markdownURL + ")"
	})

	out = strings.ReplaceAll(out, zeroWidthSpace, "")

	return out
}

func (converter *DefaultConverter) convertLinks(ticketID int64, wikiPage string, in string) string {
	out := in

	// conversion occurs in three distinct phases with each phase dealing with one part of the link syntax
	// and leaving the remainder for the next stage
	out = converter.convertBrackettedTracLinks(wikiPage, out)
	out = converter.convertUnbrackettedTracLinks(ticketID, wikiPage, out)
	out = converter.unmarkLinks(out)
	return out
}

var httpLinkDisguiseRegexp = regexp.MustCompile(`(https?)://`)
var httpLinkUndisguiseRegexp = regexp.MustCompile(`(https?):@@`)

// disguiseLinks temporarily disguises links into a format that doesn't interfere with other Trac -> markdown regexps
// - in particular the '//' in 'http(s)://...' clashes with Trac's '//' italics marker
func (converter *DefaultConverter) disguiseLinks(in string) string {
	out := in
	out = httpLinkDisguiseRegexp.ReplaceAllString(out, `$1:@@`)
	return out
}

// undisguiseLink converts temporarily "disguised" links back to their correct format.
func (converter *DefaultConverter) undisguiseLinks(in string) string {
	out := in
	out = httpLinkUndisguiseRegexp.ReplaceAllString(out, `$1://`)
	return out
}
