// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/stevejefferson/trac2gitea/importer"
	"github.com/stevejefferson/trac2gitea/markdown"

	"github.com/spf13/pflag"
	"github.com/stevejefferson/trac2gitea/accessor/gitea"
	"github.com/stevejefferson/trac2gitea/accessor/trac"
	"github.com/stevejefferson/trac2gitea/log"
)

var dbOnly bool
var wikiOnly bool
var wikiPush bool
var overwrite bool
var verbose bool
var wikiConvertPredefineds bool
var generateMaps bool
var tracRootDir string
var giteaRootDir string
var giteaMainConfigPath string
var giteaDefaultUser string
var giteaOrg string
var giteaRepo string
var userMapInputFile string
var userMapOutputFile string
var labelMapInputFile string
var labelMapOutputFile string
var revisionMapFile string
var giteaWikiRepoURL string
var giteaWikiRepoToken string
var giteaWikiRepoDir string

// parseArgs parses the command line arguments, populating the variables above.
func parseArgs() {
	giteaMainConfigPathParam := pflag.String("app-ini", "",
		"Path to Gitea configuration file (app.ini). If not set, fetch the configuration from the standard locations. "+
			"Useful if Gitea is running in a Docker container and you need a separate configuration file to reference the data on the host volumes.")
	giteaDefaultUserParam := pflag.String("default-user", "",
		"Fallback Gitea user if a Trac user cannot be mapped to an existing Gitea user. Defaults to <gitea-org>")
	wikiURLParam := pflag.String("wiki-url", "",
		"URL of wiki repository - defaults to <server-root-url>/<gitea-user>/<gitea-repo>.wiki.git")
	wikiTokenParam := pflag.String("wiki-token", "",
		"password/token for accessing wiki repository (ignored if wiki-url provided)")
	wikiDirParam := pflag.String("wiki-dir", "",
		"directory into which to checkout (clone) wiki repository - defaults to cwd")
	wikiConvertPredefinedsParam := pflag.Bool("wiki-convert-predefined", false,
		"convert Trac predefined wiki pages - by default we skip these")

	generateMapsParam := pflag.Bool("generate-maps", false,
		"generate default user/label mappings into provided map files (note: no conversion will be performed in this case)")
	dbOnlyParam := pflag.Bool("db-only", false,
		"convert database only")
	wikiOnlyParam := pflag.Bool("wiki-only", false,
		"convert wiki only")
	wikiNoPushParam := pflag.Bool("no-wiki-push", false,
		"do not push wiki on completion")
	overwriteParam := pflag.Bool("overwrite", false,
		"overwrite existing data (by default previously-imported issues, labels, wiki pages etc are skipped)")
	verboseParam := pflag.Bool("verbose", false,
		"verbose output")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"Usage: %s [options] <trac-root> <gitea-root> <gitea-org> <gitea-repo> [<user-map>] [<label-map>] [<revision-map>]\n",
			os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	verbose = *verboseParam
	overwrite = *overwriteParam
	dbOnly = *dbOnlyParam
	wikiOnly = *wikiOnlyParam
	wikiPush = !*wikiNoPushParam
	generateMaps = *generateMapsParam

	if dbOnly && wikiOnly {
		log.Fatal("cannot generate only database AND only wiki!")
	}
	wikiConvertPredefineds = *wikiConvertPredefinedsParam
	giteaWikiRepoURL = *wikiURLParam
	giteaWikiRepoToken = *wikiTokenParam
	giteaWikiRepoDir = *wikiDirParam
	giteaMainConfigPath = *giteaMainConfigPathParam

	if (pflag.NArg() < 4) || (pflag.NArg() > 7) {
		pflag.Usage()
		os.Exit(1)
	}

	tracRootDir = pflag.Arg(0)
	giteaRootDir = pflag.Arg(1)
	giteaOrg = pflag.Arg(2)
	giteaRepo = pflag.Arg(3)
	if pflag.NArg() > 4 {
		userMapFile := pflag.Arg(4)
		if generateMaps {
			userMapOutputFile = userMapFile
		} else {
			userMapInputFile = userMapFile
		}
	}

	if pflag.NArg() > 5 {
		labelMapFile := pflag.Arg(5)
		if generateMaps {
			labelMapOutputFile = labelMapFile
		} else {
			labelMapInputFile = labelMapFile
		}
	}

	if pflag.NArg() > 6 {
		revisionMapFile = pflag.Arg(6)
	}

	if giteaDefaultUser = *giteaDefaultUserParam; giteaDefaultUser == "" {
		giteaDefaultUser = giteaOrg
	}
}

// importData imports the non-wiki Trac data.
func importData(dataImporter *importer.Importer, userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap, revisionMap map[string]string) error {
	var err error
	if err = dataImporter.ImportFullNames(); err != nil {
		return err
	}
	if err = dataImporter.ImportComponents(componentMap); err != nil {
		return err
	}
	if err = dataImporter.ImportPriorities(priorityMap); err != nil {
		return err
	}
	if err = dataImporter.ImportResolutions(resolutionMap); err != nil {
		return err
	}
	if err = dataImporter.ImportSeverities(severityMap); err != nil {
		return err
	}
	if err = dataImporter.ImportTypes(typeMap); err != nil {
		return err
	}
	if err = dataImporter.ImportKeywords(keywordMap); err != nil {
		return err
	}
	if err = dataImporter.ImportVersions(versionMap); err != nil {
		return err
	}
	if err = dataImporter.ImportMilestones(); err != nil {
		return err
	}
	if err = dataImporter.ImportTickets(userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap, revisionMap); err != nil {
		return err
	}

	return nil
}

// performImport performs the actual import
func performImport(dataImporter *importer.Importer, userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap, revisionMap map[string]string) error {
	if !wikiOnly {
		if err := importData(dataImporter, userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap, revisionMap); err != nil {
			dataImporter.RollbackImport()
			return err
		}
	}

	if !dbOnly {
		if err := dataImporter.ImportWiki(); err != nil {
			dataImporter.RollbackImport()
			return err
		}
	}

	return dataImporter.CommitImport()
}

// createImporter creates and configures the importer
func createImporter() (*importer.Importer, error) {
	tracAccessor, err := trac.CreateDefaultAccessor(tracRootDir)
	if err != nil {
		return nil, err
	}
	giteaAccessor, err := gitea.CreateDefaultAccessor(
		giteaRootDir, giteaMainConfigPath, giteaOrg, giteaRepo, giteaWikiRepoURL, giteaWikiRepoToken, giteaWikiRepoDir, overwrite, wikiPush, dbOnly)
	if err != nil {
		return nil, err
	}
	markdownConverter := markdown.CreateDefaultConverter(tracAccessor, giteaAccessor)

	dataImporter, err := importer.CreateImporter(tracAccessor, giteaAccessor, markdownConverter, giteaDefaultUser, wikiConvertPredefineds)
	if err != nil {
		return nil, err
	}

	return dataImporter, nil
}

func main() {
	parseArgs()

	var logLevel = log.INFO
	if verbose {
		logLevel = log.TRACE
	}
	log.SetLevel(logLevel)

	dataImporter, err := createImporter()
	if err != nil {
		log.Fatal("%+v", err)
		return
	}

	userMap, err := readUserMap(userMapInputFile, dataImporter)
	if err != nil {
		log.Fatal("%+v", err)
		return
	}

	componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap, err := readLabelMaps(labelMapInputFile, dataImporter)
	if err != nil {
		log.Fatal("%+v", err)
		return
	}

	if generateMaps {
		// note: no need to commit or rollback transaction here - nothing has been imported yet
		if userMapOutputFile != "" {
			if err = writeUserMapToFile(userMapOutputFile, userMap); err != nil {
				log.Fatal("%+v", err)
				return
			}
			log.Info("wrote user map to %s", userMapOutputFile)
		}
		if labelMapOutputFile != "" {
			if err = writeLabelMapsToFile(labelMapOutputFile, componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap); err != nil {
				log.Fatal("%+v", err)
				return
			}
			log.Info("wrote label map to %s", labelMapOutputFile)
		}

		return
	}

	revisionMap, err := readRevisionMap(revisionMapFile)
	if err != nil {
		log.Fatal("%+v", err)
		return
	}

	err = performImport(dataImporter, userMap, componentMap, priorityMap, resolutionMap, severityMap, typeMap, keywordMap, versionMap, revisionMap)
	if err != nil {
		log.Fatal("%+v", err)
		return
	}
}
