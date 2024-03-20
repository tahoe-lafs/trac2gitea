// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package importer

import (
	"github.com/stevejefferson/trac2gitea/accessor/gitea"
	"github.com/stevejefferson/trac2gitea/accessor/trac"
)

// importTicket imports a Trac ticket as a Gitea issue, returning the id of the created issue or gitea.NullID if the issue was not created.
func (importer *Importer) importTicket(ticket *trac.Ticket, closed bool, userMap, revisionMap map[string]string) (int64, error) {
	reporterID, err := importer.getUserID(ticket.Reporter, userMap)
	if err != nil {
		return gitea.NullID, err
	}
	// Use the "original author" migration feature if the reporter cannot be mapped onto a Gitea user
	originalAuthorName := ""
	if reporterID == gitea.NullID {
		reporterID = importer.defaultAuthorID
		originalAuthorName = ticket.Reporter
	}

	ownerID := gitea.NullID
	if ticket.Owner != "" {
		ownerID, err = importer.getUserID(ticket.Owner, userMap)
		if err != nil {
			return gitea.NullID, err
		}
	}

	convertedDescription := importer.markdownConverter.TicketConvert(ticket.TicketID, ticket.Description)
	convertedDescription = MapRevisions(convertedDescription, revisionMap)
	issue := gitea.Issue{Index: ticket.TicketID, Summary: ticket.Summary, ReporterID: reporterID,
		Milestone: ticket.MilestoneName, OriginalAuthorID: 0, OriginalAuthorName: originalAuthorName,
		Closed: closed, Description: convertedDescription, Created: ticket.Created, Updated: ticket.Updated}
	issueID, err := importer.giteaAccessor.AddIssue(&issue)
	if err != nil {
		return gitea.NullID, err
	}

	// if we have a Gitea user for the Trac ticket owner then assign the Gitea issue to that user
	if ownerID != gitea.NullID {
		err = importer.giteaAccessor.AddIssueAssignee(issueID, ownerID)
		if err != nil {
			return gitea.NullID, err
		}
	}

	// issue reporter (and issue assignee if different) are now participants in the issue
	err = importer.giteaAccessor.AddIssueParticipant(issueID, reporterID)
	if err != nil {
		return gitea.NullID, err
	}
	if ownerID != gitea.NullID && ownerID != reporterID {
		err = importer.giteaAccessor.AddIssueParticipant(issueID, ownerID)
		if err != nil {
			return gitea.NullID, err
		}
	}

	return issueID, nil
}

// ImportTickets imports Trac tickets as Gitea issues.
func (importer *Importer) ImportTickets(
	userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, versionMap, revisionMap map[string]string) error {
	err := importer.tracAccessor.GetTickets(func(ticket *trac.Ticket) error {
		closed := (ticket.Status == string(trac.TicketStatusClosed))
		issueID, err := importer.importTicket(ticket, closed, userMap, revisionMap)
		if err != nil {
			return err
		}
		if issueID == gitea.NullID {
			return nil
		}

		_, err = importer.importTicketLabel(issueID, ticket.ComponentName, componentMap)
		if err != nil {
			return err
		}

		_, err = importer.importTicketLabel(issueID, ticket.PriorityName, priorityMap)
		if err != nil {
			return err
		}

		_, err = importer.importTicketLabel(issueID, ticket.ResolutionName, resolutionMap)
		if err != nil {
			return err
		}

		_, err = importer.importTicketLabel(issueID, ticket.SeverityName, severityMap)
		if err != nil {
			return err
		}

		_, err = importer.importTicketLabel(issueID, ticket.TypeName, typeMap)
		if err != nil {
			return err
		}

		_, err = importer.importTicketLabel(issueID, ticket.VersionName, versionMap)
		if err != nil {
			return err
		}

		lastUpdate, err := importer.importTicketAttachments(ticket.TicketID, issueID, ticket.Created, userMap, revisionMap)
		if err != nil {
			return err
		}
		lastUpdate, err = importer.importTicketChanges(ticket.TicketID, issueID, lastUpdate,
			userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, versionMap,
			revisionMap)
		if err != nil {
			return err
		}

		// If closed, set closed date here to lastUpdate!
		if closed {
			if err = importer.giteaAccessor.SetIssueClosedTime(issueID, lastUpdate); err != nil {
				return err
			}
		}

		if err = importer.giteaAccessor.SetIssueUpdateTime(issueID, lastUpdate); err != nil {
			return err
		}

		if err = importer.giteaAccessor.UpdateIssueCommentCount(issueID); err != nil {
			return err
		}

		if err = importer.giteaAccessor.UpdateIssueIndex(issueID, ticket.TicketID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = importer.giteaAccessor.UpdateLabelIssueCounts()
	if err != nil {
		return err
	}

	err = importer.giteaAccessor.UpdateMilestoneIssueCounts()
	if err != nil {
		return err
	}

	return importer.giteaAccessor.UpdateRepoIssueCounts()
}
