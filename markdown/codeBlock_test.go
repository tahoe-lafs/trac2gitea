// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package markdown_test

import (
	"testing"

	"go.uber.org/mock/gomock"
)

func TestSingleLineCodeBlock(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	code := "this is some code"

	conversion := converter.WikiConvert(wikiPage, leadingText+"{{{"+code+"}}}"+trailingText)
	assertEquals(t, conversion, leadingText+"`"+code+"`"+trailingText)
}

func TestMultiLineCodeBlock(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	codeLine1 := "this is some code\n"
	codeLine2 := "this is more code\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!processor\n"+
			codeLine1+
			codeLine2+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```#!processor\n"+
			codeLine1+
			codeLine2+
			"```\n"+
			trailingText)
}

func TestNoProcessorBlock(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "this is some text\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```\n"+
			contents+
			"```\n"+
			trailingText)
}

func TestNewLineProcessorBlock(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "this is some text\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{\n"+
			"#!processor with spaces\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```#!processor with spaces\n"+
			contents+
			"```\n"+
			trailingText)
}

func TestHTMLBlock(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "<strong style=\"color: grey\">This is some raw HTML</strong>\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!html\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"\n"+
			contents+
			"\n"+
			trailingText)
}

func TestHTMLTag(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "this is some text\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!table\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"<table>\n"+
			"\n"+
			contents+
			"\n"+
			"</table>\n"+
			trailingText)
}

func TestHTMLTagWithSingleParam(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "this is some text\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!div id=\"test\"\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"<div id=\"test\">\n"+
			"\n"+
			contents+
			"\n"+
			"</div>\n"+
			trailingText)
}

func TestHTMLTagWithMultipleParams(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "this is some text\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!div class=\"test\" style=\"color: red; font-size: 90%\"\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"<div class=\"test\" style=\"color: red; font-size: 90%\">\n"+
			"\n"+
			contents+
			"\n"+
			"</div>\n"+
			trailingText)
}

func TestNestedHTML(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	head1 := "Header 1\n"
	head2 := "Header 2\n"
	content1 := "Content 1\n"
	content2 := "Content 2\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!table\n"+
			"{{{#!tr\n"+
			"{{{#!th\n"+
			head1+
			"}}}\n"+
			"{{{#!td\n"+
			content1+
			"}}}\n"+
			"}}}\n"+
			"{{{#!tr\n"+
			"{{{#!th\n"+
			head2+
			"}}}\n"+
			"{{{#!td\n"+
			content2+
			"}}}\n"+
			"}}}\n"+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"<table>\n"+
			"\n"+
			"<tr>\n"+
			"\n"+
			"<th>\n"+
			"\n"+
			head1+
			"\n"+
			"</th>\n"+
			"<td>\n"+
			"\n"+
			content1+
			"\n"+
			"</td>\n"+
			"\n"+
			"</tr>\n"+
			"<tr>\n"+
			"\n"+
			"<th>\n"+
			"\n"+
			head2+
			"\n"+
			"</th>\n"+
			"<td>\n"+
			"\n"+
			content2+
			"\n"+
			"</td>\n"+
			"\n"+
			"</tr>\n"+
			"\n"+
			"</table>\n"+
			trailingText)
}

func TestHTMLComment(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	contents := "this is some text\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!comment\n"+
			contents+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"<!---\n"+
			contents+
			"-->\n"+
			trailingText)
}

func TestCodeBlockWithCommitTicketReference(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	codeLine1 := "#!CommitTicketReference repository=\"\" revision=\"4574\"\n"
	codeLine2 := "Remove CommitTicketReference\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{"+codeLine1+
			codeLine2+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```\n"+
			codeLine2+
			"```\n"+
			trailingText)
}

func TestCodeBlockWithLanguage(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	codeLine1 := "#!cpp\n"
	codeLine2 := "This is some C++\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{"+codeLine1+
			codeLine2+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```cpp\n"+
			codeLine2+
			"```\n"+
			trailingText)
}

func TestCodeBlockWithMappedLanguage(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	codeLine1 := "#!c++\n"
	codeLine2 := "This is some C++\n"

	// NOTE: We also check \n after {{{ here
	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{\n"+codeLine1+
			codeLine2+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```cpp\n"+
			codeLine2+
			"```\n"+
			trailingText)
}

func TestNoConversionInsideCodeBlock(t *testing.T) {
	setUp(t)
	defer tearDown(t)

	codeLine1 := "Website reference: http://www.example.com\n"
	codeLine2 := "[wiki:WikiPage trac-style wiki link] followed by Trac-style //italics//\n"
	codeLine3 := "- bullet point\n"
	codeLine4 := "== Trac-style Subheading\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!processor\n"+
			codeLine1+
			codeLine2+
			codeLine3+
			codeLine4+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"```#!processor\n"+
			codeLine1+
			codeLine2+
			codeLine3+
			codeLine4+
			"```\n"+
			trailingText)
}

func TestConversionInsideHTMLBlock(t *testing.T) {
	setUp(t)
	// expect call to translate name of wiki page
	mockGiteaAccessor.
		EXPECT().
		TranslateWikiPageName(gomock.Eq("WikiPage")).
		Return("TransformedWikiPage")

	defer tearDown(t)

	tracLine1 := "Website reference: http://www.example.com\n"
	tracLine2 := "[wiki:WikiPage trac-style wiki link] followed by Trac-style //italics//\n"
	tracLine3 := "- bullet point\n"
	tracLine4 := "== Trac-style Subheading\n"

	mdLine1 := "Website reference: <http://www.example.com>\n"
	mdLine2 := "[trac-style wiki link](TransformedWikiPage) followed by Trac-style *italics*\n"
	mdLine3 := "- bullet point\n"
	mdLine4 := "## Trac-style Subheading\n"

	conversion := converter.WikiConvert(
		wikiPage,
		leadingText+"\n"+
			"{{{#!td\n"+
			tracLine1+
			tracLine2+
			tracLine3+
			tracLine4+
			"}}}\n"+
			trailingText)
	assertEquals(t, conversion,
		leadingText+"\n"+
			"<td>\n"+
			"\n"+
			mdLine1+
			mdLine2+
			mdLine3+
			mdLine4+
			"\n"+
			"</td>\n"+
			trailingText)
}
