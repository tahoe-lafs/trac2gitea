// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package wiki

import (
	"fmt"
	"strings"
	"time"

	"github.com/stevejefferson/trac2gitea/log"
	"github.com/stevejefferson/trac2gitea/markdown"

	"github.com/stevejefferson/trac2gitea/accessor/gitea"
	"github.com/stevejefferson/trac2gitea/accessor/trac"
)

// Importer imports Trac Wiki data into a Gitea wiki repository.
type Importer struct {
	tracAccessor          trac.Accessor
	giteaAccessor         gitea.Accessor
	defaultPageOwner      string
	defaultPageOwnerEMail string
	convertPredefineds    bool
}

// CreateImporter creates a Trac wiki to Gitea wiki repository importer.
func CreateImporter(
	tAccessor trac.Accessor,
	gAccessor gitea.Accessor,
	dfltPageOwner string,
	convertPredefs bool) (*Importer, error) {

	dfltPageOwnerID, err := gAccessor.GetUserID(dfltPageOwner)
	if err != nil {
		return nil, err
	}
	if dfltPageOwnerID == -1 {
		err = fmt.Errorf("Cannot find default owner %s for wiki pages to be imported from Trac", dfltPageOwner)
		log.Error("%v\n", err)
		return nil, err
	}
	dfltPageOwnerEMail, err := gAccessor.GetUserEMailAddress(dfltPageOwnerID)
	if err != nil {
		return nil, err
	}

	importer := Importer{
		tracAccessor:          tAccessor,
		giteaAccessor:         gAccessor,
		defaultPageOwner:      dfltPageOwner,
		defaultPageOwnerEMail: dfltPageOwnerEMail,
		convertPredefineds:    convertPredefs}
	return &importer, nil
}

// ImportWiki imports a Trac wiki into a Gitea wiki repository.
func (importer *Importer) ImportWiki(push bool) error {
	err := importer.giteaAccessor.CloneWiki()
	if err != nil {
		return err
	}

	importer.importWikiAttachments()
	importer.importWikiPages()

	if push {
		return importer.giteaAccessor.PushWiki()
	}

	log.Info("Trac wiki has been imported into cloned wiki repository. Please review changes and push back to remote when done.\n")
	return nil
}

func (importer *Importer) importWikiAttachments() {
	importer.tracAccessor.GetWikiAttachments(func(pageName string, filename string) error {
		tracAttachmentPath := importer.tracAccessor.GetWikiAttachmentPath(pageName, filename)
		giteaAttachmentPath := importer.giteaAccessor.GetWikiAttachmentRelPath(pageName, filename)
		return importer.giteaAccessor.CopyFileToWiki(tracAttachmentPath, giteaAttachmentPath)
	})
}

// cache of commit message list keyed by page name - use this because 'LogWiki' is potentially slow
var commitMessagesByPage = make(map[string][]string)

// pageCommitExists determines whether or not a commit of the given page exists with a commit message containing the provided string
func (importer *Importer) pageCommitExists(pageName string, commitString string) (bool, error) {
	commitMessages, haveCommitMessages := commitMessagesByPage[pageName]
	if !haveCommitMessages {
		pageCommitMessages, err := importer.giteaAccessor.LogWiki(pageName)
		if err != nil {
			return false, err
		}
		commitMessagesByPage[pageName] = pageCommitMessages
		commitMessages = pageCommitMessages
	}

	for _, commitMessage := range commitMessages {
		if strings.Contains(commitMessage, commitString) {
			return true, nil
		}
	}

	return false, nil
}

func (importer *Importer) importWikiPages() {
	importer.tracAccessor.GetWikiPages(func(pageName string, pageText string, author string, comment string, version int64, updateTime int64) error {
		// skip predefined pages
		if !importer.convertPredefineds && importer.tracAccessor.IsPredefinedPage(pageName) {
			log.Debug("Skipping predefined Trac page %s\n", pageName)
			return nil
		}

		// have we already converted this version of the trac wiki page?
		// - if so, skip it on the assumption that this is a re-import and that the only thing that is likely to have changed
		// is the addition of later trac versions of wiki pages - these will get added to the wiki repo as later versions
		tracPageVersionIdentifier := fmt.Sprintf("trac page %s (version %d)", pageName, version)
		translatedPageName := importer.giteaAccessor.TranslateWikiPageName(pageName)
		hasCommit, err := importer.pageCommitExists(translatedPageName, tracPageVersionIdentifier)
		if err != nil {
			return err
		}
		if hasCommit {
			log.Info("Wiki page %s: %s is already present in wiki - skipping...\n", translatedPageName, tracPageVersionIdentifier)
			return nil
		}

		// convert and write wiki page
		tracToMarkdownConverter := markdown.CreateWikiDefaultConverter(
			importer.tracAccessor, importer.giteaAccessor, pageName)
		markdownText := tracToMarkdownConverter.Convert(pageText)
		importer.giteaAccessor.WriteWikiPage(translatedPageName, markdownText)

		// translate Trac wiki page (version) author into a Gitea user
		giteaAuthor := importer.defaultPageOwner
		giteaAuthorEMail := importer.defaultPageOwnerEMail
		giteaAuthorID, err := importer.giteaAccessor.GetUserID(author)
		if err != nil {
			return err
		}

		if giteaAuthorID != -1 {
			giteaAuthor = author
			giteaAuthorEMail, err = importer.giteaAccessor.GetUserEMailAddress(giteaAuthorID)
			if err != nil {
				return err
			}
		}

		// commit version of wiki page to local repository
		updateTimeStr := time.Unix(updateTime, 0)
		comment = fmt.Sprintf("%s\n[Imported: %s - updated at %s by Trac user %s]\n",
			comment, tracPageVersionIdentifier, updateTimeStr, author)
		err = importer.giteaAccessor.CommitWiki(giteaAuthor, giteaAuthorEMail, comment)
		log.Info("Wiki page %s: converted from %s\n", translatedPageName, tracPageVersionIdentifier)
		return err
	})
}
