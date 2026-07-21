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
	Retired bool
}

// Projects, newest first. The first four are featured on the home page.
var Projects = []Project{
	{Year: "2026", Name: "Photo Location Recorder", Tech: "SwiftUI · SwiftData", Desc: "Keeps an on-device timeline of where you've been, so photos get the right GPS data later. Negligible battery cost.", Store: "App Store"},
	{Year: "2025", Name: "Car Model Identifier", Tech: "SwiftUI · SwiftData · CloudKit", Desc: "AI-powered vehicle model detector — point it at any car and get the model in seconds.", Store: "App Store"},
	{Year: "2024", Name: "BreezeCheck", Tech: "watchOS · Swift Charts", Desc: "Standalone Apple Watch app for pollen and air quality, with real-time data from Open-Meteo.", Store: "App Store"},
	{Year: "2022", Name: "Relâmpagos em Portugal", Tech: "SwiftUI · Core Data · Swift Charts", Desc: "Near real-time lightning for Iberia, the Atlantic islands and northern Africa, with push notifications and on-device analysis.", Store: "App Store"},
	{Year: "2021", Name: "Bird Sound Identifier", Tech: "Core ML · BirdNET", Desc: "On-device birdsong recognition covering 6,362 species. 2,100+ sales, 4.8★ across 290 reviews.", Store: "App Store"},
	{Year: "2020", Name: "QR Peek", Tech: "ARKit · CloudKit · SwiftUI", Desc: "Create, display and share QR content in augmented reality, pinned to any surface.", Store: "App Store"},
	{Year: "2018–20", Name: "Fogos em Portugal", Tech: "iOS · Android · Java API", Desc: "Real-time wildfire tracking for Portugal — custom Java backend, push notifications, lightning and fire-risk overlays.", Retired: true},
	{Year: "2017–18", Name: "Today in History", Tech: "iOS · watchOS · Android · .NET API", Desc: "Historical events for any day, on your phone or wrist. Native UI on each platform, self-hosted Wikipedia API.", Store: "App Store"},
}
