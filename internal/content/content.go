// Package content holds static software-project data. Journal posts and the
// synchronized Flickr gallery live in the database.
package content

// Project is one entry on the /software page.
type Project struct {
	Year    string
	Name    string
	Tech    string
	Desc    string
	Store   string // link label, e.g. "App Store"; empty when retired
	URL     string // store page; the link is hidden when empty
	Retired bool
}

// Projects, newest first. The first four are featured on the home page.
var Projects = []Project{
	{Year: "2026", Name: "Photo Location Recorder", Tech: "SwiftUI · SwiftData", Desc: "Records where you've been using the Significant Changes API, so you can geotag photos from cameras without GPS later, directly in the app or by exporting a GPX for Lightroom.", Store: "App Store", URL: "https://apps.apple.com/app/id6760829092"},
	{Year: "2025", Name: "Car Model Identifier", Tech: "SwiftUI · SwiftData · CloudKit", Desc: "Point the camera at a car and it tells you the make and model.", Store: "App Store", URL: "https://apps.apple.com/app/id6743397573"},
	{Year: "2024", Name: "BreezeCheck", Tech: "watchOS · Swift Charts", Desc: "Standalone Apple Watch app for pollen and air quality, with real-time data from Open-Meteo.", Store: "App Store", URL: "https://apps.apple.com/app/id6496851031"},
	{Year: "2022", Name: "Relâmpagos em Portugal", Tech: "SwiftUI · Core Data · Swift Charts", Desc: "Near real-time lightning for Iberia, the Atlantic islands and northern Africa, with push notifications and on-device analysis.", Store: "App Store", URL: "https://apps.apple.com/app/id6443856846"},
	{Year: "2021", Name: "Bird Sound Identifier", Tech: "Core ML · BirdNET", Desc: "Identifies birds by their song, on-device, using BirdNET. Covers 6,362 species, with over 2,100 sales and 4.8★ from 290 reviews.", Store: "App Store", URL: "https://apps.apple.com/app/id1541890005"},
	{Year: "2020", Name: "QR Peek", Tech: "ARKit · CloudKit · SwiftUI", Desc: "Create and share QR codes and see their content in AR, pinned to any surface.", Store: "App Store", URL: "https://apps.apple.com/app/id1516308939"},
	{Year: "2018–20", Name: "Fogos em Portugal", Tech: "iOS · Android · Java API", Desc: "Real-time wildfire tracking for Portugal, with my own Java backend, push notifications and lightning and fire-risk overlays.", Retired: true},
	{Year: "2017–18", Name: "Today in History", Tech: "iOS · watchOS · Android · .NET API", Desc: "Historical events for any day on iPhone, Apple Watch and Android. Native UI on each platform and a self-hosted Wikipedia API.", Store: "App Store", URL: "https://apps.apple.com/app/id1443549588"},
}
