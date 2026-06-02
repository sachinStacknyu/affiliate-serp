package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/joho/godotenv"
	serpapi "github.com/serpapi/serpapi-golang"
)

type SerpAPIResponse struct {
	SearchMetadata SearchMetadata `json:"search_metadata"`
	Properties     []Property     `json:"properties"`
}
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

	CategoryToken string `json:"category_token"`
	SerpAPILink   string `json:"serpapi_link"`
}
type AffiliateLinkRequest struct {
	Trs     int    `json:"trs"`
	Marker  int    `json:"marker"`
	Shorten bool   `json:"shorten"`
	Links   []Link `json:"links"`
}
type Link struct {
	URL string `json:"url"`
	// SubID string `json:"sub_id,omitempty"`  optional value inside the travelpayout
}
type AffiliateLinkResponse struct {
	Result struct {
		Trs     int  `json:"trs"`
		Marker  int  `json:"marker"`
		Shorten bool `json:"shorten"`
		Links   []struct {
			URL        string `json:"url"`
			Code       string `json:"code"`
			PartnerURL string `json:"partner_url"`
		} `json:"links"`
	} `json:"result"`

	Code   string `json:"code"`
	Status int    `json:"status"`
}
type Flex[T any] struct {
	Value T
}

func (f *Flex[T]) UnmarshalJSON(data []byte) error {

	var v T
	if err := json.Unmarshal(data, &v); err == nil {
		f.Value = v
		return nil
	}

	// string fallback -> Int
	var s string
	if err := json.Unmarshal(data, &s); err == nil {

		switch any(f.Value).(type) {
		// string fallback -> Int
		case int:
			match := regexp.MustCompile(`^\d+`).FindString(s)
			if match != "" {
				n, err := strconv.Atoi(match)
				if err == nil {
					f.Value = any(n).(T)
				}
			}
			// string fallback -> Int64
		case int64:
			match := regexp.MustCompile(`^\d+`).FindString(s)
			if match != "" {
				n, err := strconv.ParseInt(
					match,
					10,
					64,
				)
				if err == nil {
					f.Value = any(n).(T)
				}
			}
			// string fallback -> Float
		case float64:
			match := regexp.MustCompile(
				`^(\d+(\.\d+)?)`,
			).FindString(s)

			if match != "" {
				n, err := strconv.ParseFloat(
					match,
					64,
				)
				if err == nil {
					f.Value = any(n).(T)
				}
			}
			// string fallback -> String
		case string:
			f.Value = any(s).(T)
		}

		return nil
	}

	return nil
}
func (f Flex[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Value)
}
func fetchHotels(query, location, checkIn, checkOut, apiKey, gl, currency, googleDomain string) (*SerpAPIResponse, error) {

	setting := serpapi.NewSerpApiClientSetting(apiKey)
	setting.Engine = "google_hotels"

	client := serpapi.NewClient(setting)

	params := map[string]string{
		"q":             query,
		"location":      location,
		"hl":            "en",
		"gl":            gl,
		"google_domain": googleDomain,
		"currency":      currency,

		"check_in_date":  checkIn,
		"check_out_date": checkOut,
	}

	results, err := client.Search(params)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}

	var response SerpAPIResponse

	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Convert the Original_link to affiliate link
func ConvertTheToAffiliateLink(trs int, marker int, propertyURL string, token string) string {
	reqBody := AffiliateLinkRequest{
		Trs:     trs,
		Marker:  marker,
		Shorten: true,
		Links: []Link{
			{
				URL: propertyURL,
			},
		},
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return propertyURL
	}
	req, err := http.NewRequest(
		"POST",
		"https://api.travelpayouts.com/links/v1/create",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return propertyURL
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Token", token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return propertyURL
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return propertyURL
	}
	var affiliateResp AffiliateLinkResponse
	err = json.Unmarshal(body, &affiliateResp)
	if err != nil {
		return propertyURL
	}
	if len(affiliateResp.Result.Links) > 0 {
		return affiliateResp.Result.Links[0].PartnerURL
	}
	return propertyURL
}

func hotelsHandler(w http.ResponseWriter, r *http.Request) {

	apiKey := os.Getenv("SERP_API_KEY")
	if apiKey == "" {
		http.Error(
			w,
			"SERP_API_KEY missing",
			http.StatusInternalServerError,
		)
		return
	}

	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		query = "Hotels"
	}

	location := q.Get("location")
	if location == "" {
		location = "Ahmedabad, Gujarat, India"
	}

	checkIn := q.Get("check_in")
	if checkIn == "" {
		checkIn = "2026-06-01"
	}

	checkOut := q.Get("check_out")
	if checkOut == "" {
		checkOut = "2026-06-02"
	}
	gl := q.Get("gl")
	if gl == "" {
		gl = "in" // default to India
	}

	currency := q.Get("currency")
	if currency == "" {
		currency = "INR" // default to Indian Rupee
	}

	googleDomain := q.Get("google_domain")
	if googleDomain == "" {
		googleDomain = "google.co.in" // default to Indian Google domain
	}

	data, err := fetchHotels(
		query,
		location,
		checkIn,
		checkOut,
		apiKey,
		gl,
		currency,
		googleDomain,
	)

	if err != nil {
		log.Println(err)
		http.Error(
			w,
			"failed to fetch hotels",
			http.StatusBadGateway,
		)
		return
	}

	// Convert hotel links to affiliate links
	travelToken := os.Getenv("TRAVELPAYOUTS_TOKEN")
	trsInt, _ := strconv.Atoi(os.Getenv("TP_TRS"))
	markerInt, _ := strconv.Atoi(os.Getenv("TP_MARKER"))

	for i, hotel := range data.Properties {
		original := hotel.Link
		affiliate := ConvertTheToAffiliateLink(
			int(trsInt),
			int(markerInt),
			hotel.Link,
			travelToken,
		)
		//  for example to check if it actaully convert to affiliate link for not (Works perfect)
		// affiliate := ConvertTheToAffiliateLink(
		// 	trsInt,
		// 	markerInt,
		// 	"https://www.booking.com/",
		// 	travelToken,
		// )

		// fmt.Println("Affiliate:", affiliate)

		fmt.Println("Original:", original)
		fmt.Println("Affiliate:", affiliate)
		data.Properties[i].Link = affiliate
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println(err)
	}
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
	http.HandleFunc(
		"/hotels",
		hotelsHandler,
	)

	port := "8080"
	fmt.Println(
		"Server running at http://localhost:" + port,
	)
	log.Fatal(
		http.ListenAndServe(
			":"+port,
			nil,
		),
	)
}
