// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package trac

import "github.com/stevejefferson/trac2gitea/log"

// GetComponentNames retrieves all Trac component names, passing each one to the provided "handler" function.
func (accessor *DefaultAccessor) GetComponentNames(handlerFn func(cmptName string) error) error {
	rows, err := accessor.db.Query(`SELECT name FROM component`)
	if err != nil {
		log.Error("Problem retrieving names of trac components: %v\n", err)
		return err
	}

	for rows.Next() {
		var cmptName string
		if err := rows.Scan(&cmptName); err != nil {
			log.Error("Problem extracting name of trac component: %v\n", err)
			return err
		}

		err = handlerFn(cmptName)
		if err != nil {
			return err
		}
	}

	return nil
}
