// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package importer

import (
	"fmt"

	"github.com/stevejefferson/trac2gitea/log"

	"github.com/stevejefferson/trac2gitea/accessor/trac"
)

func (importer *Importer) importWikiAttachments() {
	importer.tracAccessor.GetWikiAttachments(func(attachment *trac.WikiAttachment) error {
		tracAttachmentPath := importer.tracAccessor.GetWikiAttachmentPath(attachment)
		giteaAttachmentPath := importer.giteaAccessor.GetWikiAttachmentRelPath(attachment.PageName, attachment.FileName)
		return importer.giteaAccessor.CopyFileToWiki(tracAttachmentPath, giteaAttachmentPath)
	})
}

func (importer *Importer) importWikiPages() {
	importer.tracAccessor.GetWikiPages(func(page *trac.WikiPage) error {
		// skip predefined pages
		if !importer.convertPredefineds && importer.tracAccessor.IsPredefinedPage(page.Name) {
			log.Debug("skipping predefined Trac page %s", page.Name)
			return nil
		}

		// have we already converted this version of the trac wiki page?
		// - if so, skip it on the assumption that this is a re-import and that the only thing that is likely to have changed
		// is the addition of later trac versions of wiki pages - these will get added to the wiki repo as later versions
		tracPageVersionIdentifier := fmt.Sprintf("[Imported from Trac: page %s, version %d]", page.Name, page.Version)
		translatedPageName := importer.giteaAccessor.TranslateWikiPageName(page.Name)

		// convert and write wiki page
		markdownText := importer.markdownConverter.WikiConvert(page.Name, page.Text)
		written, err := importer.giteaAccessor.WriteWikiPage(translatedPageName, markdownText, tracPageVersionIdentifier)
		if err != nil {
			return err
		}
		if !written {
			log.Info("Trac wiki page %s, version %d is already present in Gitea wiki - ignored", translatedPageName, page.Version)
			return nil
		}

		// commit version of wiki page to local repository
		fullComment := tracPageVersionIdentifier + "\n\n" + page.Comment
		err = importer.giteaAccessor.CommitWikiToRepo(page.Author, page.UpdateTime, fullComment)
		log.Info("wiki page %s: converted from Trac page %s, version %d", translatedPageName, page.Name, page.Version)
		return err
	})
}

// ImportWiki imports a Trac wiki into a Gitea wiki repository.
func (importer *Importer) ImportWiki() error {
	err := importer.giteaAccessor.CloneWiki()
	if err != nil {
		return err
	}

	importer.importWikiAttachments()
	importer.importWikiPages()

	return nil
}
