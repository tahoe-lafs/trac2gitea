// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package markdown

import (
	"regexp"
	"strings"

	"github.com/stevejefferson/trac2gitea/log"
)

// Code blocks are disguised with {@{@{...}@}@} in order to avoid Trac syntax conversion inside
var nonCodeBlockRegexp = regexp.MustCompile(`(?m)(?:}@}@}$|\A)(?s)(.+?)(?:^{@{@{|\z)`)

var singleLineCodeBlockRegexp = regexp.MustCompile(`{@{@{([^\n]+?)}@}@}`)

var codeBlockBoundaryRegexp = regexp.MustCompile(`{{{|}}}`)
var tracProcessorRegexp = regexp.MustCompile(`^\s*#!(\w[^\n]*)?`)

// We support block-style HTML tags, for which we add an empty line between the tags
// and the content which might be Markdown
var htmlTags = []string{"div", "td", "th", "tr", "table"}

var codeLangs = []string{"c", "c++", "ps1", "php", "py", "sh", "cpp", "pl"}
var langMap = map[string]string{"c++": "cpp"}

// Parse code blocks delimited by triple curly brackets, converting them to raw HTML if applicable
// Else, convert them to disguised code block boundaries allowing us to preserve their contents
// See WikiProcessors for the Trac syntax details
func (converter *DefaultConverter) parseCodeBlocks(in string, accumulated []string) string {

	// This function receives a remaining `in` text and a list of currently opened code blocks

	// Do nothing if there are no curly braces in the remaining text. There shouldn't remain any pending triple brackets.
	if !codeBlockBoundaryRegexp.MatchString(in) {
		if len(accumulated) > 0 {
			log.Error("Parsing issue during markdown conversion of code blocks. Some opening triple brackets are not closed.")
		}

		return in
	}

	// Split the text around the next boundary.
	surroundings := codeBlockBoundaryRegexp.Split(in, 2)
	beforeBoundary := surroundings[0]
	afterBoundary := surroundings[1]
	convertedBoundary := ""
	newAccumulated := accumulated

	// Deal with a new block opening
	if codeBlockBoundaryRegexp.FindString(in) == "{{{" {
		// Default values for unknown processors as well as simple {{{
		convertedBoundary = "{@{@{"
		blockType := ""
		tracProcessor := ""

		// Identify the processor if there is one after the opening
		if match := tracProcessorRegexp.FindStringSubmatch(afterBoundary); match != nil {
			tracProcessor = match[1]
			afterBoundary = tracProcessorRegexp.Split(afterBoundary, 2)[1]
		}
		// get rid of CommitTicketReference processors
		if strings.HasPrefix(tracProcessor, "CommitTicketReference") {
			tracProcessor = ""
		}
		// (in which case it will be kept after the converted opening by default)
		if tracProcessor != "" {
			convertedBoundary += "#!" + tracProcessor
		}

		// If the processor is a known language, do not keep the #! mark
		// Convert the lang to the supported gitea version if needed
		for _, codeLang := range codeLangs {
			if strings.TrimSpace(tracProcessor) == codeLang {
				lang := codeLang
				if fixedLang, found := langMap[lang]; found {
					lang = fixedLang
				}
				convertedBoundary = "{@{@{" + lang
			}
		}

		// If it is a supported html tag, replace the opening by a <tag>
		// followed by an empty line
		for _, tag := range htmlTags {
			if strings.HasPrefix(tracProcessor, tag) {
				blockType = tag
				convertedBoundary = "<" + tracProcessor + ">\n"
				break
			}
		}

		// If it is a comment, replace the opening by the start of an HTML comment
		if strings.HasPrefix(tracProcessor, "comment") || strings.HasPrefix(tracProcessor, "htmlcomment") {
			blockType = "comment"
			convertedBoundary = "<!---"
		}

		// If it is a #!html processor, replace the block by an empty line to preserve functionality
		// in Markdown (this must be checked after #!htmlcomment)
		if strings.HasPrefix(tracProcessor, "html") {
			blockType = "html"
			convertedBoundary = ""
		}

		// Remember that we opened a new block
		newAccumulated = append(accumulated, blockType)
	}

	// Deal with a block closing
	if codeBlockBoundaryRegexp.FindString(in) == "}}}" {
		// We are closing the lastly opened block. If we are not inside one, do not convert or disguise these braces.
		if len(accumulated) == 0 {
			return beforeBoundary + "}}}" + converter.parseCodeBlocks(afterBoundary, accumulated)
		}
		blockType := accumulated[len(accumulated)-1]

		// By default, this will become a closing triple bracket
		convertedBoundary = "}@}@}"

		// If we are closing a supported html tag, replace the closing by an empty line and a </tag>
		for _, tag := range htmlTags {
			if blockType == tag {
				convertedBoundary = "\n</" + tag + ">"
			}
		}

		// If we are closing a comment, replace the closing by the end of an HTML comment
		if blockType == "comment" {
			convertedBoundary = "-->"
		}

		// If we are closing a #!html processor, just remove the block marks
		if blockType == "html" {
			convertedBoundary = ""
		}

		// This block is now closed
		newAccumulated = accumulated[:len(accumulated)-1]
	}

	// Continue parsing the rest of the text using recursion
	// This is guaranteed to end as `afterBoundary` is strictly shorter than `in`
	return beforeBoundary + convertedBoundary + converter.parseCodeBlocks(afterBoundary, newAccumulated)
}

func (converter *DefaultConverter) convertCodeBlocks(in string) string {

	// convert all {{{...}}} to html or to disguised triple brackets, depending on
	// specified Trac processors (see WikiProcessors)
	acc := []string{}
	return converter.parseCodeBlocks(in, acc)
}

func (converter *DefaultConverter) undisguiseCodeBlocks(in string) string {

	// convert single line {{{...}}} to `...`
	out := singleLineCodeBlockRegexp.ReplaceAllString(in, "`$1`")

	// convert other brackets to ```
	out = strings.ReplaceAll(out, "{@{@{", "```")
	out = strings.ReplaceAll(out, "}@}@}", "```")

	return out
}

func (converter *DefaultConverter) convertNonCodeBlocks(in string, convertFn func(string) string) string {
	return nonCodeBlockRegexp.ReplaceAllStringFunc(in, convertFn)
}
