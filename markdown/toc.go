// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package markdown

import "regexp"

var tocRegexp = regexp.MustCompile(`\[\[TOC\]\]\n+`)

func (converter *DefaultConverter) removeTOC(in string) string {
	// Remove [[TOC]] special mark
	return tocRegexp.ReplaceAllString(in, "")
}
