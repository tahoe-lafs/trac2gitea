// Copyright 2020 Steve Jefferson. All rights reserved.
// Use of this source code is governed by a GPL-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	systemlog "log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"
	"github.com/stevejefferson/trac2gitea/log"
	"gopkg.in/src-d/go-git.v4"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DefaultAccessor is the default implementation of the gitea Accessor interface, accessing Gitea directly via its database and filestore.
type DefaultAccessor struct {
	rootDir       string
	mainConfig    *ini.File
	customConfig  *ini.File
	db            *gorm.DB
	dbType        string
	userName      string
	repoName      string
	repoID        int64
	wikiRepoURL   string
	wikiRepoToken string
	wikiRepoDir   string
	wikiRepo      *git.Repository
	overwrite     bool
	pushWiki      bool
	dbOnly        bool
}

func fetchConfig(configPath string) (*ini.File, error) {
	_, err := os.Stat(configPath)
	if err != nil {
		err = errors.Wrapf(err, "looking for config file %s", configPath)
		return nil, nil
	}

	config, err := ini.Load(configPath)
	if err != nil {
		err = errors.Wrapf(err, "loading config file %s", configPath)
		return nil, err
	}

	return config, nil
}

// CreateDefaultAccessor returns a new Gitea default accessor.
func CreateDefaultAccessor(
	giteaRootDir string,
	giteaMainConfigPath string,
	giteaUserName string,
	giteaRepoName string,
	giteaWikiRepoURL string,
	giteaWikiRepoToken string,
	giteaWikiRepoDir string,
	overwriteData bool,
	pushWiki bool,
	dbOnly bool) (*DefaultAccessor, error) {
	stat, err := os.Stat(giteaRootDir)
	if err != nil {
		err = errors.Wrapf(err, "looking for root directory %s of Gitea instance", giteaRootDir)
		return nil, err
	}
	if !stat.IsDir() {
		err = fmt.Errorf("gitea root path %s is not a directory", giteaRootDir)
		return nil, err
	}

	// If a Gitea app.ini file was passed from the command line, use that. Otherwise,
	// use the previous behaviour (search expected paths)
	var giteaMainConfig, giteaCustomConfig *ini.File
	var giteaCustomConfigPath string

	if giteaMainConfigPath == "" {
		// Search standard locations
		giteaMainConfigPath = "/etc/gitea/app.ini"
		giteaMainConfig, err = fetchConfig(giteaMainConfigPath)
		if err != nil {
			return nil, err
		}

		giteaCustomConfigPath = fmt.Sprintf("%s/conf/app.ini", giteaRootDir)
		giteaCustomConfig, err = fetchConfig(giteaCustomConfigPath)
		if err != nil {
			return nil, err
		}
	} else {
		// Passed from command line
		giteaMainConfig, err = fetchConfig(giteaMainConfigPath)
		if err != nil {
			return nil, err
		}
	}

	if giteaMainConfig == nil && giteaCustomConfig == nil {
		if giteaCustomConfigPath == "" {
			err = fmt.Errorf("cannot find Gitea config in '%s'", giteaMainConfigPath)
		} else {
			err = fmt.Errorf("cannot find Gitea config in '%s' or '%s'", giteaMainConfigPath, giteaCustomConfigPath)
		}
		return nil, err
	}

	giteaAccessor := DefaultAccessor{
		rootDir:       giteaRootDir,
		mainConfig:    giteaMainConfig,
		customConfig:  giteaCustomConfig,
		db:            nil,
		dbType:        "",
		userName:      giteaUserName,
		repoName:      giteaRepoName,
		repoID:        0,
		wikiRepoURL:   "",
		wikiRepoToken: "",
		wikiRepoDir:   "",
		wikiRepo:      nil,
		overwrite:     overwriteData,
		pushWiki:      pushWiki,
		dbOnly:        dbOnly,
	}

	dialect, dbType, err := giteaAccessor.getDbDialect()
	if err != nil {
		return nil, err
	}

	gormLogger := logger.New(
		systemlog.New(os.Stdout, "\r\n", systemlog.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  getGormLogLevel(),
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(dialect, &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		QueryFields:            true,
		Logger:                 gormLogger,
	})

	if err != nil {
		err = errors.Wrap(err, "opening Gitea database")
		return nil, err
	}

	// Start transaction
	giteaAccessor.db = db.Begin()
	giteaAccessor.dbType = dbType
	if err = giteaAccessor.db.Error; err != nil {
		err = errors.Wrap(err, "Unable to start Gitea database transaction")
	}

	giteaRepoID, err := giteaAccessor.getRepoID(giteaUserName, giteaRepoName)
	if err != nil {
		return nil, err
	}
	if giteaRepoID == NullID {
		return nil, fmt.Errorf("cannot find repository %s for user %s", giteaRepoName, giteaUserName)
	}
	giteaAccessor.repoID = giteaRepoID

	// find directory into which to clone wiki
	wikiRepoName := giteaRepoName + ".wiki"
	if giteaWikiRepoDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			err = errors.Wrapf(err, "getting cwd")
			return nil, err
		}

		giteaWikiRepoDir = filepath.Join(cwd, wikiRepoName)
	}
	if !dbOnly { // do not care about not being able to clone the wiki if we are not going to import it
		_, err = os.Stat(giteaWikiRepoDir)
		if os.IsPermission(err) {
			return nil, fmt.Errorf("you don't have permission to access directory %s", giteaWikiRepoDir)
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("wiki repository directory %s already exists", giteaWikiRepoDir)
		}
	}
	giteaAccessor.wikiRepoDir = giteaWikiRepoDir

	// find URL from which clone wiki
	if giteaWikiRepoURL == "" {
		rootURL := giteaAccessor.GetStringConfig("server", "ROOT_URL")
		if giteaWikiRepoToken != "" {
			slashSlashPos := strings.Index(rootURL, "//")
			if slashSlashPos == -1 {
				return nil, fmt.Errorf("ROOT_URL %s malformed? expecting a '//'", rootURL)
			}

			// insert username and token into URL - 'http://example.com' should become 'http://<user>:<token>@example.com'
			rootURL = rootURL[0:slashSlashPos+2] + giteaUserName + ":" + giteaWikiRepoToken + "@" + rootURL[slashSlashPos+2:]

			giteaAccessor.wikiRepoToken = giteaWikiRepoToken
		}
		if rootURL[len(rootURL)-1:] != "/" {
			rootURL = rootURL + "/"
		}
		giteaWikiRepoURL = fmt.Sprintf("%s%s/%s.git", rootURL, giteaUserName, wikiRepoName)
	}
	log.Info("using Wiki repo URL %s", giteaWikiRepoURL)
	giteaAccessor.wikiRepoURL = giteaWikiRepoURL

	return &giteaAccessor, nil
}

// Map log output to gorm log levels. Change to logger.Info to see SQL
func getGormLogLevel() logger.LogLevel {
	lvl := log.GetLevel()
	if lvl == log.TRACE {
		return logger.Info
	} else {
		return logger.Error
	}
}

func (accessor *DefaultAccessor) getDbDialect() (gorm.Dialector, string, error) {
	var dialect gorm.Dialector

	dbType := accessor.GetStringConfig("database", "DB_TYPE")
	dbName := accessor.GetStringConfig("database", "NAME")
	dbUser := accessor.GetStringConfig("database", "USER")
	dbPassword := accessor.GetStringConfig("database", "PASSWD")
	dbHost := accessor.GetStringConfig("database", "HOST")
	dbSslMode := accessor.GetStringConfig("database", "SSL_MODE")

	switch dbType {
	case "sqlite3":
		giteaDbPath := accessor.GetStringConfig("database", "PATH")
		dialect = sqlite.Open(giteaDbPath)

	case "mysql":
		if dbCharset == "utf8" || dbCharset == "" {
			dbCharset = "utf8mb4"
		}
		connstr := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=True&loc=Local",
			url.PathEscape(dbUser), url.PathEscape(dbPassword), dbHost, dbName, dbCharset)
		dialect = mysql.Open(connstr)

	case "postgres":
		connstr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			url.PathEscape(dbUser), url.PathEscape(dbPassword), dbHost, dbName, dbSslMode)
		dialect = postgres.Open(connstr)

	case "mssql":
		connstr := fmt.Sprintf("sqlserver://%s:%s@%s?database=%s",
			url.PathEscape(dbUser), url.PathEscape(dbPassword), dbHost, dbName)
		dialect = sqlserver.Open(connstr)

	default:
		return nil, "", errors.Errorf("Unknown Gitea database type, %s", dbType)
	}

	return dialect, dbType, nil
}
