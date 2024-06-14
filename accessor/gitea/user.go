// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// SetUserFullName sets the full name for a named Gitea user
func (accessor *DefaultAccessor) SetUserFullName(userName string, userFullName string) error {
	err := accessor.db.Model(&User{}).
		Where("name=?", userName).
		Limit(1).
		Update("full_name", &userFullName).Error

	if err != nil {
		err = errors.Wrapf(err, "setting full name %s for user %s", userFullName, userName)
		return err
	}

	return nil
}

// GetUserID retrieves the id of a named Gitea user - returns NullID if no such user.
func (accessor *DefaultAccessor) GetUserID(userName string) (int64, error) {
	if strings.Trim(userName, " ") == "" {
		return NullID, nil
	}

	var id int64 = NullID
	err := accessor.db.Model(&User{}).
		Where("lower_name=@name OR email=@name", map[string]interface{}{"name": userName}).
		Limit(1).
		Pluck("id", &id).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		err = errors.Wrapf(err, "retrieving id of user %s", userName)
		return NullID, err
	}

	return id, nil
}

// GetUserEMailAddress retrieves the email address of a given user
func (accessor *DefaultAccessor) GetUserEMailAddress(userName string) (string, error) {
	var emailAddress string

	err := accessor.db.Model(&User{}).
		Where("lower_name=?", userName).
		Limit(1).
		Pluck("email", &emailAddress).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		err = errors.Wrapf(err, "retrieving email address of user %s", userName)
		return "", err
	}

	return emailAddress, nil
}

// getUserRepoURL retrieves the relative URL of the current repository for the current user
func (accessor *DefaultAccessor) getUserRepoURL() string {
	return fmt.Sprintf("/%s/%s", accessor.userName, accessor.repoName)
}

// MatchUser retrieves the name of the user best matching a user name or email address
func (accessor *DefaultAccessor) MatchUser(userName string, userEmail string) (string, error) {
	if userName == "" {
		return "", nil
	}

	var matchedUserName string

	err := accessor.db.Model(&User{}).
		Where("type=0"). // Do not match users to organizations
		Where(accessor.db.
			Where("lower_name=?", strings.ToLower(userName)).
			Or("full_name=?", userName).
			Or("email=?", userEmail)).
		Limit(1).
		Pluck("lower_name", &matchedUserName).Error

	if err != nil {
		err = errors.Wrapf(err, "trying to match user name %s, email %s", userName, userEmail)
		return "", err
	}

	return matchedUserName, nil
}
