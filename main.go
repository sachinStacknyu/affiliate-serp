package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

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

	Link          string `json:"link,omitempty"`
	OriginalLink  string `json:"original_link"`
	AffiliateLink string `json:"affiliate_link"`

	RatePerNight  RateInfo    `json:"rate_per_night"`
	PriceNumeric  int         `json:"price_numeric"`
	OverallRating float64     `json:"overall_rating"`
	Reviews       int         `json:"reviews"`
	HotelClass    FlexFloat64 `json:"hotel_class"`
	NearbyPlaces  []Nearby    `json:"nearby_places"`
	PropertyToken string      `json:"property_token"`

	Thumbnail string       `json:"thumbnail"`
	Images    []HotelImage `json:"images"`
}

type HotelImage struct {
	Thumbnail string `json:"thumbnail"`
}

type RateInfo struct {
	Lowest string `json:"lowest"`
}

type Nearby struct {
	Name string `json:"name"`
}

type FlexFloat64 struct {
	Value float64
}

var leadingNumber = regexp.MustCompile(`^(\d+(\.\d+)?)`)

func (f *FlexFloat64) UnmarshalJSON(data []byte) error {

	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		f.Value = num
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	match := leadingNumber.FindString(s)
	if match == "" {
		f.Value = 0
		return nil
	}

	num, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return err
	}

	f.Value = num
	return nil
}

func (f FlexFloat64) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Value)
}

// price parser logic
var priceRegex = regexp.MustCompile(`[0-9,]+`)

func parsePrice(price string) int {

	match := priceRegex.FindString(price)
	if match == "" {
		return 0
	}

	match = strings.ReplaceAll(match, ",", "")

	value, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}

	return value
}

type TPRequest struct {
	TRS     int         `json:"trs"`
	Marker  int         `json:"marker"`
	Shorten bool        `json:"shorten"`
	Links   []TPLinkReq `json:"links"`
}

type TPLinkReq struct {
	URL string `json:"url"`
}

type TPResponse struct {
	Result struct {
		Links []struct {
			PartnerURL string `json:"partner_url"`
		} `json:"links"`
	} `json:"result"`
}

func CreateAffiliateLink(
	hotelURL string,
	trs int,
	marker int,
	token string,
) string {

	reqBody := TPRequest{
		TRS:     trs,
		Marker:  marker,
		Shorten: true,
		Links: []TPLinkReq{
			{
				URL: hotelURL,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return hotelURL
	}

	req, err := http.NewRequest(
		"POST",
		"https://api.travelpayouts.com/links/v1/create",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return hotelURL
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Token", token)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Travelpayouts error:", err)
		return hotelURL
	}
	defer resp.Body.Close()

	var tpResp TPResponse

	if err := json.NewDecoder(resp.Body).Decode(&tpResp); err != nil {
		return hotelURL
	}

	if len(tpResp.Result.Links) == 0 {
		return hotelURL
	}

	return tpResp.Result.Links[0].PartnerURL
}

func fetchHotels(
	query,
	location,
	checkIn,
	checkOut,
	apiKey string,
) (*SerpAPIResponse, error) {

	setting := serpapi.NewSerpApiClientSetting(apiKey)
	setting.Engine = "google_hotels"

	client := serpapi.NewClient(setting)

	params := map[string]string{
		"q":              query,
		"location":       location,
		"hl":             "en",
		"gl":             "us",
		"google_domain":  "google.com",
		"currency":       "INR",
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

func hotelsHandler(w http.ResponseWriter, r *http.Request) {

	apiKey := os.Getenv("SERP_API_KEY")
	tpToken := os.Getenv("TP_API_TOKEN")

	trs, _ := strconv.Atoi(os.Getenv("TP_TRS"))
	marker, _ := strconv.Atoi(os.Getenv("TP_MARKER"))

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

	minPrice, _ := strconv.Atoi(
		q.Get("min_price"),
	)

	maxPrice, _ := strconv.Atoi(
		q.Get("max_price"),
	)

	data, err := fetchHotels(
		query,
		location,
		checkIn,
		checkOut,
		apiKey,
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

	// process hotels
	processed := []Property{}

	for _, hotel := range data.Properties {

		price := parsePrice(
			hotel.RatePerNight.Lowest,
		)

		hotel.PriceNumeric = price

		// thumbnail fallback
		if hotel.Thumbnail == "" &&
			len(hotel.Images) > 0 {

			hotel.Thumbnail =
				hotel.Images[0].Thumbnail
		}

		// price filter
		if minPrice > 0 &&
			price < minPrice {
			continue
		}

		if maxPrice > 0 &&
			price > maxPrice {
			continue
		}

		// preserve SerpAPI link
		hotel.OriginalLink = hotel.Link

		// generate affiliate link
		if tpToken != "" &&
			trs != 0 &&
			marker != 0 &&
			hotel.Link != "" {

			hotel.AffiliateLink =
				CreateAffiliateLink(
					hotel.Link,
					trs,
					marker,
					tpToken,
				)
		}

		processed = append(
			processed,
			hotel,
		)
	}

	data.Properties = processed

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	json.NewEncoder(w).Encode(data)
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
		"Server running on http://localhost:" + port,
	)

	log.Fatal(
		http.ListenAndServe(
			":"+port,
			nil,
		),
	)
}
