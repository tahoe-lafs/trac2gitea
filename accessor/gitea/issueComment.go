// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/stevejefferson/trac2gitea/log"
)

// GetIssueCommentIDByTime retrieves the ID of the comment created at a given time for a given issue.
// Since different issue changes can happen at the same time, this tries to return the "comment" type
// change, or falls back to another type by increasing IssueCommentType.
func (accessor *DefaultAccessor) GetIssueCommentIDByTime(issueID int64, createdTime int64) (int64, error) {
	// Note: Trac stores timestamps with greater precision than Gitea, so it is possible multiple "comment" type
	// changes are returned in the query, but this is unlikely.
	var commentIDs = []int64{}
	err := accessor.db.Model(&IssueComment{}).
		Select("id").
		Where("issue_id=? AND created_unix=?", issueID, createdTime).
		Order("type ASC").
		Find(&commentIDs).
		Error

	if err != nil {
		err = errors.Wrapf(err, "retrieving ids of comments created at \"%s\" for issue %d", time.Unix(createdTime, 0), issueID)
		return -1, err
	}

	if len(commentIDs) == 0 {
		log.Error("could not find issue comment at %s for issue %d", time.Unix(createdTime, 0), issueID)
		return -1, nil
	}

	return commentIDs[0], nil
}

// updateIssueComment updates an existing issue comment
func (accessor *DefaultAccessor) updateIssueComment(issueCommentID int64, issueID int64, comment *IssueComment) error {
	comment.ID = issueCommentID
	comment.IssueID = issueID
	comment.CreatedTime = comment.Time

	if err := accessor.db.Save(&comment).Error; err != nil {
		return errors.Wrapf(err, "updating comment on issue %d timed at %s", issueID, time.Unix(comment.Time, 0))
	}

	log.Debug("updated issue comment at %s for issue %d (id %d)", time.Unix(comment.Time, 0), issueID, issueCommentID)

	return nil
}

// insertIssueComment adds a new comment to a Gitea issue, returns id of created comment.
func (accessor *DefaultAccessor) insertIssueComment(issueID int64, comment *IssueComment) (int64, error) {
	comment.IssueID = issueID
	comment.CreatedTime = comment.Time

	if err := accessor.db.Create(&comment).Error; err != nil {
		err = errors.Wrapf(err, "adding comment \"%s\" for issue %d", comment.Text, issueID)
		return NullID, err
	}

	log.Debug("added issue comment at %s for issue %d (id %d)", time.Unix(comment.Time, 0), issueID, comment.ID)

	return comment.ID, nil
}

// findIssueComment checks for the existence and ID of a comment with the same timestamp and change type in the given issue
func (accessor *DefaultAccessor) findIssueComment(issueID int64, createdTime int64, commentType IssueCommentType) (int64, error) {
	var commentIDs = []int64{}
	err := accessor.db.Model(&IssueComment{}).
		Select("id").
		Where("issue_id=? AND created_unix=? AND type=?", issueID, createdTime, commentType).
		Find(&commentIDs).
		Error

	if err != nil {
		err = errors.Wrapf(err, "retrieving ids of comments created at \"%s\" for issue %d", time.Unix(createdTime, 0), issueID)
		return -1, err
	}

	if len(commentIDs) == 0 {
		return -1, nil
	}

	return commentIDs[0], nil
}

// AddIssueComment adds a comment on a Gitea issue, returns id of created comment
func (accessor *DefaultAccessor) AddIssueComment(issueID int64, comment *IssueComment) (int64, error) {
	// Check whether a particular issue comment already exists (and hence whether we need to insert or update it).
	issueCommentID, err := accessor.findIssueComment(issueID, comment.CreatedTime, comment.CommentType)
	if err != nil {
		return NullID, err
	}

	if issueCommentID == -1 {
		return accessor.insertIssueComment(issueID, comment)
	}

	if accessor.overwrite {
		err := accessor.updateIssueComment(issueCommentID, issueID, comment)
		if err != nil {
			return NullID, err
		}
	} else {
		log.Info("issue %d already has comment timed at %s - ignored", issueID, time.Unix(comment.Time, 0))
	}

	return issueCommentID, nil
}

// GetIssueCommentURL retrieves the URL for viewing a Gitea comment for a given issue.
func (accessor *DefaultAccessor) GetIssueCommentURL(issueNumber int64, commentID int64) string {
	repoURL := accessor.getUserRepoURL()
	return fmt.Sprintf("%s/issues/%d#issuecomment-%d", repoURL, issueNumber, commentID)
}
