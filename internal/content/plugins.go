package content

// Plugin is one entry on the /lightroom page. Version mirrors the VERSION
// table in each plug-in's Info.lua, and DownloadURL points at the "latest"
// release permalink so it keeps working as new builds are published.
type Plugin struct {
	Name        string
	Bundle      string // bundle folder name, e.g. "iCloud Photos.lrplugin"
	Version     string
	Tagline     string
	Features    []string
	Requires    []string
	FirstRun    string // what to do after enabling it in Plug-in Manager
	RepoURL     string
	DownloadURL string
	License     string
}

// Plugins listed on /lightroom.
var Plugins = []Plugin{
	{
		Name:    "iCloud Photos",
		Bundle:  "iCloud Photos.lrplugin",
		Version: "1.5.0",
		Tagline: "A Publish Service that sends photos from Lightroom into albums in Photos, and from there into iCloud.",
		Features: []string{
			"Every published collection maps to an album in Photos. Republish updates the same album instead of piling up duplicates, and renaming the collection renames the album.",
			"Exports JPEG, JPEG XL or AVIF, with the usual quality, size, metadata, sharpening and watermark controls. JPEG XL can be lossless.",
			"Mirrors your curation: flagged photos, four stars and up, or exactly five stars become favorites in Photos.",
			"Replacing an old rendition asks once per publish, not once per photo, and skips anything that has since been moved out of the album.",
			"Publishes in batches of 32, so a few thousand photos don't pay a process launch each. One bad file gets flagged for republish instead of stopping the run.",
			"Talks only to PhotoKit on your Mac. No network, no account, no credentials.",
		},
		Requires: []string{
			"macOS 13 or newer (JPEG XL needs 14)",
			"Lightroom Classic 13 or newer",
			"iCloud Photos enabled in Photos",
		},
		FirstRun:    "In Library, add the service under Publish Services, open Publishing Manager and click Request Photos Access.",
		RepoURL:     "https://github.com/rafaelfrancisco-dev/Lightroom-iCloud-Photos",
		DownloadURL: "https://github.com/rafaelfrancisco-dev/Lightroom-iCloud-Photos/releases/latest/download/iCloud-Photos.lrplugin.zip",
		License:     "MIT",
	},
	{
		Name:    "Vision Keywords",
		Bundle:  "VisionKeywords.lrplugin",
		Version: "1.1.0",
		Tagline: "Keywords your photos with Apple's on-device Vision classifier — the same one behind search in Photos.",
		Features: []string{
			"Runs entirely on your Mac. Nothing to download, no API key, no account, no network.",
			"Reads Lightroom's embedded previews, so photos don't need exporting first. RAW files included.",
			"Keeps up with a few hundred photos a minute across roughly 1,300 labels.",
			"Files everything under an AI parent keyword that's excluded from export, so generated tags never reach your exports.",
			"Tune the confidence threshold, cap the keywords per photo, or deny labels you never want. One menu item removes everything it added.",
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
