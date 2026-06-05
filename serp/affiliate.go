package serp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/joho/godotenv"
	serpapi "github.com/serpapi/serpapi-golang"
)

type AffiliateLinkRequest struct {
	Trs     int    `json:"trs"`
	Marker  int    `json:"marker"`
	Shorten bool   `json:"shorten"`
	Links   []Link `json:"links"`
}

type Link struct {
	URL string `json:"url"`
}

type AffiliateLinkResponse struct {
	Result struct {
		Links []struct {
			URL        string `json:"url"`
			Code       string `json:"code"`
			PartnerURL string `json:"partner_url"`
		} `json:"links"`
	} `json:"result"`
	Code   string `json:"code"`
	Status int    `json:"status"`
}

// ConvertToAffiliateLink converts a hotel booking URL to a Travelpayouts
// affiliate link. Falls back to the original URL on any error.
func ConvertToAffiliateLink(trs, marker int, propertyURL, token string) string {
	body, err := json.Marshal(AffiliateLinkRequest{
		Trs:     trs,
		Marker:  marker,
		Shorten: true,
		Links:   []Link{{URL: propertyURL}},
	})
	if err != nil {
		return propertyURL
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.travelpayouts.com/links/v1/create",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return propertyURL
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Token", token)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return propertyURL
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return propertyURL
	}

	var aResp AffiliateLinkResponse
	if err := json.Unmarshal(raw, &aResp); err != nil {
		return propertyURL
	}

	if len(aResp.Result.Links) > 0 && aResp.Result.Links[0].PartnerURL != "" {
		return aResp.Result.Links[0].PartnerURL
	}
	return propertyURL
}

// ApplyAffiliateLinks rewrites all property and ad links in-place with
// Travelpayouts affiliate URLs.
func ApplyAffiliateLinks(data *SerpAPIResponse, trs, marker int, token string) {
	for i, p := range data.Properties {
		if p.Link != "" {
			data.Properties[i].Link = ConvertToAffiliateLink(trs, marker, p.Link, token)
		}
	}
	for i, ad := range data.Ads {
		if ad.Link != "" {
			data.Ads[i].Link = ConvertToAffiliateLink(trs, marker, ad.Link, token)
		}
	}
}

type SerpAPIResponse struct {
	SearchMetadata    SearchMetadata `json:"search_metadata"`
	SearchInformation map[string]any `json:"search_information"`
	Ads               []Ad           `json:"ads"`
	Properties        []Property     `json:"properties"`
}

// PropertyDetailResponse is used for property-detail mode (single property).
// SerpAPI returns the property fields at the top level, not inside a "properties" array.
type PropertyDetailResponse struct {
	SearchMetadata    SearchMetadata `json:"search_metadata"`
	SearchInformation map[string]any `json:"search_information"`

	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Link        string `json:"link"`

	PropertyToken       string            `json:"property_token"`
	GPSCoordinates      GPSCoordinates    `json:"gps_coordinates"`
	CheckInTime         string            `json:"check_in_time"`
	CheckOutTime        string            `json:"check_out_time"`
	RatePerNight        RateInfo          `json:"rate_per_night"`
	TotalRate           RateInfo          `json:"total_rate"`
	NearbyPlaces        []NearbyPlace     `json:"nearby_places"`
	HotelClass          Flex[string]      `json:"hotel_class"`
	ExtractedHotelClass Flex[float64]     `json:"extracted_hotel_class"`
	Images              []HotelImage      `json:"images"`
	OverallRating       float64           `json:"overall_rating"`
	Reviews             int               `json:"reviews"`
	Ratings             []Rating          `json:"ratings"`
	LocationRating      float64           `json:"location_rating"`
	ReviewsBreakdown    []ReviewBreakdown `json:"reviews_breakdown"`
	Amenities           []string          `json:"amenities"`
	EcoCertified        bool              `json:"eco_certified"`
}

// ToSerpAPIResponse normalises a single property detail into the standard list
// response shape so callers always receive the same JSON structure.
func (d *PropertyDetailResponse) ToSerpAPIResponse() *SerpAPIResponse {
	p := Property{
		Type:                d.Type,
		Name:                d.Name,
		Description:         d.Description,
		Link:                d.Link,
		PropertyToken:       d.PropertyToken,
		GPSCoordinates:      d.GPSCoordinates,
		CheckInTime:         d.CheckInTime,
		CheckOutTime:        d.CheckOutTime,
		RatePerNight:        d.RatePerNight,
		TotalRate:           d.TotalRate,
		NearbyPlaces:        d.NearbyPlaces,
		HotelClass:          d.HotelClass,
		ExtractedHotelClass: d.ExtractedHotelClass,
		Images:              d.Images,
		OverallRating:       d.OverallRating,
		Reviews:             d.Reviews,
		Ratings:             d.Ratings,
		LocationRating:      d.LocationRating,
		ReviewsBreakdown:    d.ReviewsBreakdown,
		Amenities:           d.Amenities,
		EcoCertified:        d.EcoCertified,
	}
	return &SerpAPIResponse{
		SearchMetadata:    d.SearchMetadata,
		SearchInformation: d.SearchInformation,
		Properties:        []Property{p},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared domain types
// ─────────────────────────────────────────────────────────────────────────────

type SearchMetadata struct {
	ID             string        `json:"id"`
	Status         string        `json:"status"`
	TotalTimeTaken TimeTakenInfo `json:"total_time_taken"`
}

type TimeTakenInfo struct {
	HumanReadable string  `json:"human_readable"`
	Raw           float64 `json:"raw"`
}

type Property struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Link                string `json:"link"`
	PropertyToken       string `json:"property_token"`
	PropertyDetailsLink string `json:"serpapi_property_details_link"`

	GPSCoordinates GPSCoordinates `json:"gps_coordinates"`

	CheckInTime  string `json:"check_in_time"`
	CheckOutTime string `json:"check_out_time"`

	RatePerNight RateInfo `json:"rate_per_night"`
	TotalRate    RateInfo `json:"total_rate"`

	Deal            Flex[string] `json:"deal"`
	DealDescription Flex[string] `json:"deal_description"`

	NearbyPlaces []NearbyPlace `json:"nearby_places"`

	HotelClass          Flex[string]  `json:"hotel_class"`
	ExtractedHotelClass Flex[float64] `json:"extracted_hotel_class"`

	Images []HotelImage `json:"images"`

	OverallRating float64 `json:"overall_rating"`
	Reviews       int     `json:"reviews"`

	Ratings []Rating `json:"ratings"`

	LocationRating float64 `json:"location_rating"`

	ReviewsBreakdown []ReviewBreakdown `json:"reviews_breakdown"`

	Amenities []string `json:"amenities"`

	EcoCertified bool `json:"eco_certified"`

	SerpAPIReviewsLink string `json:"serpapi_google_hotels_reviews_link"`
	SerpAPIPhotosLink  string `json:"serpapi_google_hotels_photos_link"`
}

type GPSCoordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type RateInfo struct {
	Lowest                   string `json:"lowest"`
	ExtractedLowest          int    `json:"extracted_lowest"`
	BeforeTaxesFees          string `json:"before_taxes_fees"`
	ExtractedBeforeTaxesFees int    `json:"extracted_before_taxes_fees"`
}

type NearbyPlace struct {
	Name            string           `json:"name"`
	Transportations []Transportation `json:"transportations"`
}

type Transportation struct {
	Type     string `json:"type"`
	Duration string `json:"duration"`
}

type HotelImage struct {
	Thumbnail     string `json:"thumbnail"`
	OriginalImage string `json:"original_image"`
}

type Rating struct {
	Stars int `json:"stars"`
	Count int `json:"count"`
}

type ReviewBreakdown struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	TotalMentioned int    `json:"total_mentioned"`
	Positive       int    `json:"positive"`
	Negative       int    `json:"negative"`
	Neutral        int    `json:"neutral"`
	CategoryToken  string `json:"category_token"`
	SerpAPILink    string `json:"serpapi_link"`
}

type Ad struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	SourceIcon string `json:"source_icon"`
	Link       string `json:"link"`

	PropertyToken       string `json:"property_token"`
	PropertyDetailsLink string `json:"serpapi_property_details_link"`

	GPSCoordinates GPSCoordinates `json:"gps_coordinates"`

	HotelClass    Flex[string] `json:"hotel_class"`
	Thumbnail     string       `json:"thumbnail"`
	OverallRating float64      `json:"overall_rating"`
	Reviews       int          `json:"reviews"`

	Price          string  `json:"price"`
	ExtractedPrice float64 `json:"extracted_price"`

	Amenities        []string `json:"amenities"`
	FreeCancellation bool     `json:"free_cancellation"`
}

// Flex handles JSON fields that may arrive as a native type OR a string.
type Flex[T any] struct {
	Value T
}

func (f *Flex[T]) UnmarshalJSON(data []byte) error {
	var v T
	if err := json.Unmarshal(data, &v); err == nil {
		f.Value = v
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		switch any(f.Value).(type) {
		case int:
			if m := regexp.MustCompile(`^\d+`).FindString(s); m != "" {
				if n, err := strconv.Atoi(m); err == nil {
					f.Value = any(n).(T)
				}
			}
		case int64:
			if m := regexp.MustCompile(`^\d+`).FindString(s); m != "" {
				if n, err := strconv.ParseInt(m, 10, 64); err == nil {
					f.Value = any(n).(T)
				}
			}
		case float64:
			if m := regexp.MustCompile(`^(\d+(\.\d+)?)`).FindString(s); m != "" {
				if n, err := strconv.ParseFloat(m, 64); err == nil {
					f.Value = any(n).(T)
				}
			}
		case string:
			f.Value = any(s).(T)
		}
	}
	return nil
}

func (f Flex[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Value)
}

// ─────────────────────────────────────────────────────────────────────────────
// Search options
// ─────────────────────────────────────────────────────────────────────────────

// HotelSearchOptions holds all parameters for both search and detail mode.
// Set PropertyToken to activate detail mode; filter fields are ignored in that case.
type HotelSearchOptions struct {
	APIKey       string
	CheckIn      string
	CheckOut     string
	GL           string
	Currency     string
	GoogleDomain string

	// Search-mode only
	Query            string
	Location         string
	HotelClass       string
	MinPrice         string
	MaxPrice         string
	PropertyTypes    string
	Amenities        string
	Rating           string
	Brands           string
	FreeCancellation string
	SpecialOffers    string
	Bedrooms         string

	// Detail-mode only (mutually exclusive with search filters above)
	PropertyToken string
}

// ─────────────────────────────────────────────────────────────────────────────
// Public entry point
// ─────────────────────────────────────────────────────────────────────────────

// FetchHotels dispatches to either hotel-list or property-detail mode.
// NewClient returns a SerpApiClient value (not a pointer) per the SDK contract.
func FetchHotels(opts HotelSearchOptions) (*SerpAPIResponse, error) {
	setting := serpapi.NewSerpApiClientSetting(opts.APIKey)
	setting.Engine = "google_hotels"
	client := serpapi.NewClient(setting) // returns serpapi.SerpApiClient (value, not pointer)

	if opts.PropertyToken != "" {
		return fetchPropertyDetail(client, opts)
	}
	return fetchHotelList(client, opts)
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal fetch helpers
// ─────────────────────────────────────────────────────────────────────────────

// fetchHotelList performs a standard hotel search returning multiple properties.
// Accepts serpapi.SerpApiClient by value, matching what NewClient returns.
func fetchHotelList(client serpapi.SerpApiClient, opts HotelSearchOptions) (*SerpAPIResponse, error) {
	resolvedLocation, err := resolveLocation(opts.Location)
	if err != nil {
		log.Printf("location resolve warning: %v, using raw: %s", err, opts.Location)
		resolvedLocation = opts.Location
	}
	log.Printf("Resolved location: %s", resolvedLocation)

	params := cleanParams(map[string]string{
		"q":                 opts.Query + " in " + resolvedLocation,
		"location":          resolvedLocation,
		"hl":                "en",
		"gl":                opts.GL,
		"google_domain":     opts.GoogleDomain,
		"currency":          opts.Currency,
		"check_in_date":     opts.CheckIn,
		"check_out_date":    opts.CheckOut,
		"hotel_class":       opts.HotelClass,
		"min_price":         opts.MinPrice,
		"max_price":         opts.MaxPrice,
		"property_types":    opts.PropertyTypes,
		"amenities":         opts.Amenities,
		"rating":            opts.Rating,
		"brands":            opts.Brands,
		"free_cancellation": opts.FreeCancellation,
		"special_offers":    opts.SpecialOffers,
		"bedrooms":          opts.Bedrooms,
	})
	log.Printf("Hotel-list params: %+v", params)

	return searchAndDecode[SerpAPIResponse](client, params)
}

// fetchPropertyDetail fetches details for a single property using property_token.
//
// Key SerpAPI behaviours:
//   - property_token drives the response; most other params are ignored.
//   - "q" must still be present — known SerpAPI bug, marked wontfix.
//   - The response is a SINGLE property at the TOP LEVEL, not inside a
//     "properties" array, so we decode into PropertyDetailResponse then normalise.
func fetchPropertyDetail(client serpapi.SerpApiClient, opts HotelSearchOptions) (*SerpAPIResponse, error) {
	log.Printf("Property-detail mode: token=%s", opts.PropertyToken)

	params := cleanParams(map[string]string{
		"q":              "hotel", // required even in detail mode (SerpAPI wontfix bug)
		"property_token": opts.PropertyToken,
		"check_in_date":  opts.CheckIn,
		"check_out_date": opts.CheckOut,
		"currency":       opts.Currency,
		"hl":             "en",
		"gl":             opts.GL,
		"google_domain":  opts.GoogleDomain,
	})
	log.Printf("Property-detail params: %+v", params)

	detail, err := searchAndDecode[PropertyDetailResponse](client, params)
	if err != nil {
		return nil, err
	}

	if detail.Name == "" {
		return nil, fmt.Errorf("property_token returned no data — token may be invalid or expired: %s", opts.PropertyToken)
	}

	return detail.ToSerpAPIResponse(), nil
}

// searchAndDecode runs a SerpAPI search and unmarshals the result into T.
func searchAndDecode[T any](client serpapi.SerpApiClient, params map[string]string) (*T, error) {
	results, err := client.Search(params)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	log.Printf("SerpAPI raw response: %s", string(raw))

	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// cleanParams removes empty-string values so SerpAPI isn't confused by them.
func cleanParams(params map[string]string) map[string]string {
	out := make(map[string]string, len(params))
	for k, v := range params {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// resolveLocation resolves a city name to SerpAPI's canonical location string,
// preferring Indian locations.
func resolveLocation(cityName string) (string, error) {
	apiURL := fmt.Sprintf(
		"https://serpapi.com/locations.json?q=%s&limit=5",
		url.QueryEscape(cityName),
	)

	resp, err := http.Get(apiURL) //nolint:noctx
	if err != nil {
		return cityName, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return cityName, err
	}

	var locations []struct {
		Name        string `json:"name"`
		CountryCode string `json:"country_code"`
	}
	if err := json.Unmarshal(body, &locations); err != nil {
		return cityName, err
	}

	log.Printf("Locations API: %d results for '%s'", len(locations), cityName)
	for _, l := range locations {
		log.Printf("  → %s (%s)", l.Name, l.CountryCode)
	}

	for _, l := range locations {
		if l.CountryCode == "IN" {
			return l.Name, nil
		}
	}
	if len(locations) > 0 {
		return locations[0].Name, nil
	}
	return cityName, fmt.Errorf("no location found for: %s", cityName)
}

// OrDefault returns val if non-empty, otherwise def.
func OrDefault(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

func hotelsHandler(w http.ResponseWriter, r *http.Request) {
	const serpAPIKey = "ae8ab742e8f27b920de320108a6bf1387611e2b4be0b03d79603185b1d07dda3"
	const travelToken = "6c2cd5864e5385f6edcbf5e2668f686b"
	const trsInt = 123456
	const markerInt = 789012

	q := r.URL.Query()

	opts := HotelSearchOptions{
		APIKey:       serpAPIKey,
		CheckIn:      OrDefault(q.Get("check_in"), "2026-06-01"),
		CheckOut:     OrDefault(q.Get("check_out"), "2026-06-02"),
		GL:           OrDefault(q.Get("gl"), "in"),
		Currency:     OrDefault(q.Get("currency"), "INR"),
		GoogleDomain: OrDefault(q.Get("google_domain"), "google.co.in"),

		Query:            OrDefault(q.Get("q"), "Hotels"),
		Location:         OrDefault(q.Get("location"), "Ahmedabad, Gujarat, India"),
		HotelClass:       q.Get("hotel_class"),
		MinPrice:         q.Get("min_price"),
		MaxPrice:         q.Get("max_price"),
		PropertyTypes:    q.Get("property_types"),
		Amenities:        q.Get("amenities"),
		Rating:           q.Get("rating"),
		Brands:           q.Get("brands"),
		FreeCancellation: q.Get("free_cancellation"),
		SpecialOffers:    q.Get("special_offers"),
		Bedrooms:         q.Get("bedrooms"),

		PropertyToken: q.Get("property_token"),
	}

	data, err := FetchHotels(opts)
	if err != nil {
		log.Printf("FetchHotels error: %v", err)
		http.Error(w, "failed to fetch hotels: "+err.Error(), http.StatusBadGateway)
		return
	}

	ApplyAffiliateLinks(data, trsInt, markerInt, travelToken)

	log.Printf("Properties: %d  Ads: %d", len(data.Properties), len(data.Ads))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("encode error:", err)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	http.HandleFunc("/hotels", hotelsHandler)

	port := "8080"
	fmt.Println("Server running at http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
