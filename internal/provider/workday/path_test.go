package workday

import "testing"

func TestJobDetailKeyFromPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		externalPath string
		wantLocation string
		wantTitle    string
		wantOK       bool
	}{
		{
			name:         "typical two-segment path",
			externalPath: "/job/US-CA-Remote/Software-Engineer_JR12345",
			wantLocation: "US-CA-Remote",
			wantTitle:    "Software-Engineer_JR12345",
			wantOK:       true,
		},
		{
			name:         "title containing extra double-dashes still cuts on the first slash",
			externalPath: "/job/US-CA-Remote/Software-Engineer--CUDA_JR12345",
			wantLocation: "US-CA-Remote",
			wantTitle:    "Software-Engineer--CUDA_JR12345",
			wantOK:       true,
		},
		{
			name:         "missing /job/ prefix is rejected",
			externalPath: "US-CA-Remote/Software-Engineer_JR12345",
			wantOK:       false,
		},
		{
			name:         "other leading segment is rejected",
			externalPath: "/details/US-CA-Remote/Software-Engineer_JR12345",
			wantOK:       false,
		},
		{
			name:         "no second segment fails",
			externalPath: "/job/onlyonesegment",
			wantOK:       false,
		},
		{
			name:         "trailing slash (empty titleSlug) fails",
			externalPath: "/job/US-CA-Remote/",
			wantOK:       false,
		},
		{
			name:         "empty location fails",
			externalPath: "/job//Software-Engineer_JR12345",
			wantOK:       false,
		},
		{
			name:         "extra path segments fail instead of percent-encoding the slash",
			externalPath: "/job/US/CA/Software-Engineer_JR12345",
			wantOK:       false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotLocation, gotTitle, gotOK := JobDetailKeyFromPath(tc.externalPath)
			if gotLocation != tc.wantLocation || gotTitle != tc.wantTitle || gotOK != tc.wantOK {
				t.Errorf("JobDetailKeyFromPath(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.externalPath, gotLocation, gotTitle, gotOK, tc.wantLocation, tc.wantTitle, tc.wantOK)
			}
		})
	}
}

func TestPublicSiteURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{
			name:    "NVIDIA's tenant shape",
			baseURL: "https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite",
			want:    "https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite",
		},
		{
			name:    "a different tenant/pod/site",
			baseURL: "https://acme.wd3.myworkdayjobs.com/wday/cxs/acme/AcmeCareers",
			want:    "https://acme.wd3.myworkdayjobs.com/AcmeCareers",
		},
		{
			name:    "trailing slash on base URL",
			baseURL: "https://acme.wd3.myworkdayjobs.com/wday/cxs/acme/AcmeCareers/",
			want:    "https://acme.wd3.myworkdayjobs.com/AcmeCareers",
		},
		{
			name:    "no path segments",
			baseURL: "https://acme.wd3.myworkdayjobs.com/",
			wantErr: true,
		},
		{
			name:    "unparseable URL",
			baseURL: "://not-a-url",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := PublicSiteURL(tc.baseURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("PublicSiteURL(%q) = %q, nil; want error", tc.baseURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("PublicSiteURL(%q) unexpected error: %v", tc.baseURL, err)
			}
			if got != tc.want {
				t.Errorf("PublicSiteURL(%q) = %q, want %q", tc.baseURL, got, tc.want)
			}
		})
	}
}
