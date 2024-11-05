// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package trac

import (
	"slices"
	"strings"

	"github.com/pkg/errors"
)

// GetKeywords retrieves all keywords used in Trac tickets, passing each one to the provided "handler" function.
func (accessor *DefaultAccessor) GetKeywords(handlerFn func(tracKeyword *Label) error) error {
	rows, err := accessor.db.Query(`SELECT DISTINCT keywords FROM ticket`)
	if err != nil {
		err = errors.Wrapf(err, "retrieving Trac keywords")
		return err
	}
	// Parse each row for multiple keywords separates by coma or spaces
	var keywords []string
	for rows.Next() {
		var rawKeywords string
		if err := rows.Scan(&rawKeywords); err != nil {
			err = errors.Wrapf(err, "retrieving Trac keywords")
			return err
		}
		rowKeywords := strings.Fields(strings.ReplaceAll(rawKeywords, ",", " "))
		// fmt.Println("Keywords:", rowKeywords)
		for j := 0; j < len(rowKeywords); j++ {
			if !slices.Contains(keywords, rowKeywords[j]) {
				keywords = append(keywords, rowKeywords[j])
			}
		}
	}

	for i := 0; i < len(keywords); i++ {
		keywordName := keywords[i]
		tracKeyword := Label{Name: keywordName, Description: ""}
		if err = handlerFn(&tracKeyword); err != nil {
			return err
		}
	}

	return nil
}
