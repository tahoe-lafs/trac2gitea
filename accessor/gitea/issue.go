// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/stevejefferson/trac2gitea/log"
	"gorm.io/gorm"
)

// GetIssueID retrieves the id of the Gitea issue corresponding to a given issue index - returns NullID if no such issue.
func (accessor *DefaultAccessor) GetIssueID(issueIndex int64) (int64, error) {
	var id int64 = NullID
	err := accessor.db.Model(&Issue{}).
		Where("repo_id=?", accessor.repoID).
		// Since index is a reserved word in some DBs
		Where(map[string]interface{}{"index": issueIndex}).
		Limit(1).
		Pluck("id", &id).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		err = errors.Wrapf(err, "retrieving issue with index %d", issueIndex)
		return NullID, err
	}

	return id, nil
}

// updateIssue updates an existing issue in Gitea
func (accessor *DefaultAccessor) updateIssue(issueID int64, issue *Issue) error {
	milestoneID, err := accessor.GetMilestoneID(issue.Milestone)
	if err != nil {
		return err
	}

	issue.ID = issueID
	issue.RepoID = accessor.repoID
	issue.MilestoneID = milestoneID

	if err := accessor.db.Save(&issue).Error; err != nil {
		return errors.Wrapf(err, "updating issue with index %d", issue.Index)
	}

	log.Info("updated issue %d: %s", issue.Index, issue.Summary)

	return nil
}

// insertIssue adds a new issue to Gitea, returns id of added issue.
func (accessor *DefaultAccessor) insertIssue(issue *Issue) (int64, error) {
	milestoneID, err := accessor.GetMilestoneID(issue.Milestone)
	if err != nil {
		return NullID, err
	}

	issue.RepoID = accessor.repoID
	issue.MilestoneID = milestoneID

	if err := accessor.db.Create(&issue).Error; err != nil {
		err = errors.Wrapf(err, "adding issue with index %d", issue.Index)
		return NullID, err
	}

	log.Info("created issue %d: %s", issue.Index, issue.Summary)

	return issue.ID, nil
}

// AddIssue adds a new issue to Gitea.
func (accessor *DefaultAccessor) AddIssue(issue *Issue) (int64, error) {
	issueID, err := accessor.GetIssueID(issue.Index)
	if err != nil {
		return NullID, err
	}

	if issueID == NullID {
		return accessor.insertIssue(issue)
	}

	if accessor.overwrite {
		err = accessor.updateIssue(issueID, issue)
		if err != nil {
			return NullID, err
		}
	} else {
		log.Info("issue %d already exists - ignored", issue.Index)
	}

	return issueID, nil
}

// SetIssueClosedTime sets the date/time a given Gitea issue was closed.
func (accessor *DefaultAccessor) SetIssueClosedTime(issueID int64, updateTime int64) error {
	if err := accessor.db.Model(&Issue{}).
		Where("id=?", issueID).
		Update("closed_unix", accessor.Greatest("closed_unix,?", updateTime)).
		Error; err != nil {

		return errors.Wrapf(err, "setting closed time for issue %d", issueID)
	}

	return nil
}

// SetIssueUpdateTime sets the update time on a given Gitea issue.
func (accessor *DefaultAccessor) SetIssueUpdateTime(issueID int64, updateTime int64) error {
	if err := accessor.db.Model(&Issue{}).
		Where("id=?", issueID).
		Update("updated_unix", accessor.Greatest("updated_unix,?", updateTime)).
		Error; err != nil {

		return errors.Wrapf(err, "setting updated time for issue %d", issueID)
	}

	return nil
}

// GetIssueURL retrieves a URL for viewing a given issue
func (accessor *DefaultAccessor) GetIssueURL(issueID int64) string {
	repoURL := accessor.getUserRepoURL()
	return fmt.Sprintf("%s/issues/%d", repoURL, issueID)
}

// UpdateIssueCommentCount updates the count of comments a given issue
func (accessor *DefaultAccessor) UpdateIssueCommentCount(issueID int64) error {
	if err := accessor.db.Model(&Issue{}).
		Where("id=?", issueID).
		Update("num_comments", accessor.
			db.Model(&IssueComment{}).
			Where("issue_id=?", issueID).
			Select("count(id)")).
		Error; err != nil {

		return errors.Wrapf(err, "updating number of comments for issue %d", issueID)
	}

	return nil
}

// UpdateIssueIndex updates the issue_index table after adding a new issue
func (accessor *DefaultAccessor) UpdateIssueIndex(issueID, ticketID int64) error {
	var issueIndex IssueIndex

	// FIXME: Why is issueID passed in at all?
	err := accessor.db.First(&issueIndex, accessor.repoID).Error
	if err != nil && err == gorm.ErrRecordNotFound {
		err = accessor.db.Create(&IssueIndex{RepoID: accessor.repoID, MaxIndex: ticketID}).Error
	} else if err == nil {
		err = accessor.db.Model(&issueIndex).
			Update("max_index", accessor.Greatest("max_index,?", ticketID)).
			Error
	}

	return err
}

// UpdateIssueDescription updates the description of an existing issue in Gitea
func (accessor *DefaultAccessor) UpdateIssueDescription(issueID int64, issueDescription string) error {
	if err := accessor.db.Model(&Issue{}).
		Where("id=?", issueID).
		Update("content", issueDescription).
		Error; err != nil {

		return errors.Wrapf(err, "updating description for issue %d", issueID)
	}

	log.Info("updated description of issue %d", issueID)

	return nil
}
