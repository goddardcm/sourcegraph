// Package appconf holds app configuration. The global config in this
// package is consulted by many app handlers, and if running in CLI
// mode, the config's values are set based on the CLI flags provided.
//
// This package is separate from package app to avoid import cycles
// when internal subpackages import it.
package appconf

import (
	"html/template"
	"time"

	"src.sourcegraph.com/sourcegraph/sgx/cli"
)

// Flags configure the app. The values are set by CLI flags (or during testing).
var Flags struct {
	NoAutoBuild bool `long:"app.no-auto-build" description:"disable automatic building of repositories from the UI"`

	ShowLatestBuiltCommit bool `long:"app.show-latest-built-commit" description:"show the latest built commit instead of the HEAD commit on a branch"`

	RepoBadgesAndCounters bool `long:"app.repo-badges-counters" description:"enable repo badges and counters"`

	DisableDirDefs bool `long:"app.disable-dir-defs" description:"do not show defs in each file/dir in repo tree viewer (slower for large repos)"`

	DisableRepoTreeSearch bool `long:"app.disable-repo-tree-search" description:"do not show repo fulltext search results (only defs) (slower for large repos)"`

	DisableGlobalSearch bool `long:"app.disable-global-search" description:"if set, only allow searching within a single repository at a time"`

	DisableSearch bool `long:"app.disable-search" description:"if set, search will be entirely disabled / never allowed"`

	DisableApps bool `long:"app.disable-apps" description:"if set, disable the changes and issues applications"`

	DisableIntegrations bool `long:"app.disable-integrations" description:"disable integrations with third-party services that are accessible from the user settings page"`

	DisableCloneURL bool `long:"app.disable-clone-url" description:"if set, disable display of the git clone URL"`

	EnableGitHubRepoShortURIAliases bool `long:"app.enable-github-repo-short-uri-aliases" description:"if set, redirect 'user/repo' URLs (with no 'github.com/') to '/github.com/user/repo'"`

	EnableGitHubStyleUserPaths bool `long:"app.enable-github-style-user-paths" description:"redirect GitHub paths like '/user' to valid ones like '/~user' (disables single-path repos)"`

	CustomLogo template.HTML `long:"app.custom-logo" description:"custom logo to display in the top nav bar (HTML)"`

	CustomNavLayout template.HTML `long:"app.custom-nav-layout" description:"custom layout to display in place of the search form (HTML)"`

	MOTD template.HTML `long:"app.motd" description:"show a custom message to all users beneath the top nav bar (HTML)" env:"SG_NAV_MSG"`

	GoogleAnalyticsTrackingID string `long:"app.google-analytics-tracking-id" description:"Google Analytics tracking ID (UA-########-#)" env:"GOOGLE_ANALYTICS_TRACKING_ID"`

	HeapAnalyticsID string `long:"app.heap-analytics-id" description:"Heap Analytics ID" env:"HEAP_ANALYTICS_ID"`

	CustomFeedbackForm template.HTML `long:"app.custom-feedback-form" description:"custom feedback form to display (HTML)" env:"CUSTOM_FEEDBACK_FORM"`

	CheckForUpdates time.Duration `long:"app.check-for-updates" description:"rate at which to check for updates and display a notification (not download/install) (0 to disable)" default:"0"`

	Blog bool `long:"app.blog" description:"Enable the Sourcegraph blog, must also set $SG_TUMBLR_API_KEY"`

	DisableExternalLinks bool `long:"app.disable-external-links" description:"Disable links to external websites"`

	ReloadAssets bool `long:"reload" description:"(development mode only) reload app templates and other assets on each request"`
}

func init() {
	cli.PostInit = append(cli.PostInit, func() {
		cli.Serve.AddGroup("App", "App flags", &Flags)
	})
}
