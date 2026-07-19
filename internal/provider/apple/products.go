package apple

// Product is one Apple products-and-services filter option.
type Product struct {
	Code string
	Name string
}

// Products is the products-and-services filter taxonomy. Unlike teams it
// cannot be listed anonymously (the refData products endpoint requires a
// signed-in user), so this is a snapshot of the list embedded in the
// server-rendered search page, captured 2026-07-19.
var Products = []Product{
	{Code: "IAD", Name: "Apple Ads"},
	{Code: "AIRPD", Name: "AirPods"},
	{Code: "APPAS", Name: "Apple accessories"},
	{Code: "APPAC", Name: "AppleCare"},
	{Code: "APPMU", Name: "Apple Music"},
	{Code: "APPAY", Name: "Apple Pay"},
	{Code: "APRTS", Name: "Apple Retail Store"},
	{Code: "APPTV", Name: "Apple TV"},
	{Code: "AWB", Name: "Apple Online Store"},
	{Code: "AVPRO", Name: "Apple Vision Pro"},
	{Code: "APPWT", Name: "Apple Watch"},
	{Code: "APPST", Name: "App Store"},
	{Code: "BTSAA", Name: "Beats Audio Accessories"},
	{Code: "CONPRO", Name: "Consumer and Pro Apps"},
	{Code: "FM", Name: "Claris"},
	{Code: "HMPD", Name: "HomePod"},
	{Code: "ICLD", Name: "iCloud"},
	{Code: "IOS", Name: "iOS"},
	{Code: "IPAD", Name: "iPad"},
	{Code: "IPHN", Name: "iPhone"},
	{Code: "IPOD", Name: "iPod"},
	{Code: "ITUNS", Name: "iTunes"},
	{Code: "MAC", Name: "Mac"},
	{Code: "MACOS", Name: "macOS"},
	{Code: "MAPS", Name: "Maps"},
	{Code: "NEWS", Name: "News"},
	{Code: "SIRI", Name: "Siri"},
	{Code: "TVOS", Name: "tvOS"},
	{Code: "VISOS", Name: "visionOS"},
	{Code: "WTOS", Name: "watchOS"},
	{Code: "OTH", Name: "Other"},
}
