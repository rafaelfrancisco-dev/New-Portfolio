package content

// Plugin is one entry on the /lightroom page. Version mirrors the VERSION
// table in each plug-in's Info.lua, and DownloadURL points at the "latest"
// release permalink so it keeps working as new builds are published.
type Plugin struct {
	Name        string
	Icon        string // which mark to draw, see pluginMark in the views
	Version     string
	Tagline     string
	Features    []string
	Requires    []string
	FirstRun    string // what to do after enabling it in Plug-in Manager
	RepoURL     string
	DownloadURL string
	License     string
}

// Plugins listed on /lightroom. Features are deliberately fragments: the page
// is a shop window, and each entry links to a README that carries the detail.
var Plugins = []Plugin{
	{
		Name:    "iCloud Photos",
		Icon:    "icloud",
		Version: "1.5.0",
		Tagline: "A Publish Service that sends photos into albums in Photos, and from there into iCloud.",
		Features: []string{
			"Each published collection maps to an album; republishing updates it in place.",
			"Exports JPEG, JPEG XL or AVIF, with the usual export controls.",
			"Flags and star ratings carry over as favorites in Photos.",
			"Publishes in batches of 32, and one bad file doesn't stop the rest.",
		},
		Requires: []string{
			"macOS 13 or newer (JPEG XL needs 14)",
			"Lightroom Classic 13 or newer",
			"iCloud Photos enabled in Photos",
		},
		FirstRun:    "Add the service under Publish Services, then click Request Photos Access in Publishing Manager.",
		RepoURL:     "https://github.com/rafaelfrancisco-dev/Lightroom-iCloud-Photos",
		DownloadURL: "https://github.com/rafaelfrancisco-dev/Lightroom-iCloud-Photos/releases/latest/download/iCloud-Photos.lrplugin.zip",
		License:     "MIT",
	},
	{
		Name:    "Vision Keywords",
		Icon:    "vision",
		Version: "1.1.0",
		Tagline: "Adds keywords to photos using Apple's on-device Vision classifier, the same one behind search in Photos.",
		Features: []string{
			"Reads Lightroom's embedded previews, so nothing needs to be exported first (RAW files included).",
			"Processes a few hundred photos a minute, with around 1,300 possible labels.",
			"Nests everything under an AI keyword that's excluded from export.",
			"You can set the confidence threshold, limit keywords per photo and block labels. One menu item removes everything it added.",
		},
		Requires: []string{
			"macOS 10.15 or newer",
			"Lightroom Classic",
		},
		FirstRun:    "Select photos in Library, then Plug-in Extras → Classify Photos with AI Keywords.",
		RepoURL:     "https://github.com/rafaelfrancisco-dev/Lightroom-Keyword-Classifier",
		DownloadURL: "https://github.com/rafaelfrancisco-dev/Lightroom-Keyword-Classifier/releases/latest/download/VisionKeywords.lrplugin.zip",
		License:     "MIT",
	},
}
