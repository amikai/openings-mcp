package ultipro

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

//go:embed testdata/search_rsp.json
var mockSearchRsp []byte

//go:embed testdata/search_filtered_rsp.json
var mockSearchFilteredRsp []byte

//go:embed testdata/filters_rsp.json
var mockFiltersRsp []byte

//go:embed testdata/view_more_locations_rsp.json
var mockViewMoreLocationsRsp []byte

//go:embed testdata/view_more_categories_rsp.json
var mockViewMoreCategoriesRsp []byte

//go:embed testdata/detail_rsp.html
var mockDetailRsp []byte

//go:embed testdata/detail_not_found_rsp.html
var mockDetailNotFoundRsp []byte

// MockCompanyCode/MockBoardID/MockOpportunityID/MockNotFoundOpportunityID
// identify the fixtures captured from TechnoServe's live board (see
// testdata/*.hurl). MockUnknownCompanyCode is deliberately absent from any
// roster, matching testdata/search_unknown_company_req.hurl's 404.
const (
	MockCompanyCode           = "TEC1006TESER"
	MockBoardID               = "18180d88-ced0-4361-bd09-d5eef66dab24"
	MockOpportunityID         = "0b81b2f5-ffe3-4604-93b8-19d912c2424f"
	MockNotFoundOpportunityID = "00000000-0000-0000-0000-000000000000"
	MockUnknownCompanyCode    = "NOSUCHCOMPANYXYZ"
	// MockFilteredCategoryID is the Finance category id used in the
	// filtered-search fixture (testdata/search_filtered_rsp.json).
	MockFilteredCategoryID = "d3bcb1ec-cbcc-4681-8111-cbd37e2f7ae6"
)

// NewMockServer returns an httptest.Server serving canned UltiPro fixture
// responses for MockCompanyCode/MockBoardID, so tests never hit the live
// API. The caller owns the server and must Close it.
func NewMockServer() *httptest.Server {
	mux := http.NewServeMux()
	prefix := "/" + MockCompanyCode + "/JobBoard/" + MockBoardID

	mux.HandleFunc(prefix+"/JobBoardView/LoadSearchResults", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OpportunitySearch struct {
				Filters []struct {
					FieldName int      `json:"fieldName"`
					Values    []string `json:"values"`
				} `json:"Filters"`
			} `json:"opportunitySearch"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, f := range body.OpportunitySearch.Filters {
			if f.FieldName == 5 && len(f.Values) > 0 && f.Values[0] == MockFilteredCategoryID {
				serveMockJSON(mockSearchFilteredRsp)(w, r)
				return
			}
		}
		serveMockJSON(mockSearchRsp)(w, r)
	})
	mux.HandleFunc("/"+MockUnknownCompanyCode+"/JobBoard/"+MockBoardID+"/JobBoardView/LoadSearchResults", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc(prefix+"/JobBoardView/GetFilters", serveMockJSON(mockFiltersRsp))
	mux.HandleFunc(prefix+"/JobBoardViewMore/ViewMorePhysicalLocations", serveMockJSON(mockViewMoreLocationsRsp))
	mux.HandleFunc(prefix+"/JobBoardViewMore/ViewMoreJobCategories", serveMockJSON(mockViewMoreCategoriesRsp))

	mux.HandleFunc(prefix+"/OpportunityDetail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Query().Get("opportunityId") == MockOpportunityID {
			w.Write(mockDetailRsp)
			return
		}
		w.Write(mockDetailNotFoundRsp)
	})

	return httptest.NewServer(mux)
}

func serveMockJSON(data []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	}
}
