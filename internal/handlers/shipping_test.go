package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsShippingInquiry(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedResult  bool
		expectedCountry string
	}{
		{
			name:            "ship to USA",
			message:         "Can you ship to USA?",
			expectedResult:  true,
			expectedCountry: "USA",
		},
		{
			name:            "shipping to Ecuador",
			message:         "What about shipping to Ecuador?",
			expectedResult:  true,
			expectedCountry: "Ecuador",
		},
		{
			name:            "delivery question",
			message:         "How long does delivery to Canada take?",
			expectedResult:  true,
			expectedCountry: "Canada",
		},
		{
			name:            "send to UK",
			message:         "Can you send to UK?",
			expectedResult:  true,
			expectedCountry: "UK",
		},
		{
			name:            "arrival time",
			message:         "When will my order arrive in Japan?",
			expectedResult:  true,
			expectedCountry: "Japan",
		},
		{
			name:            "not a shipping question",
			message:         "What holsters do you have for Glock 19?",
			expectedResult:  false,
			expectedCountry: "",
		},
		{
			name:            "product question",
			message:         "Tell me about your tactical gear",
			expectedResult:  false,
			expectedCountry: "",
		},
		{
			name:            "shipping without country",
			message:         "Do you offer shipping?",
			expectedResult:  true,
			expectedCountry: "your country",
		},
		{
			name:            "empty message",
			message:         "",
			expectedResult:  false,
			expectedCountry: "",
		},
		{
			name:            "ship to multiple countries mentioned",
			message:         "Can you ship to USA or Canada?",
			expectedResult:  true,
			expectedCountry: "USA", // Should pick first match
		},
		{
			name:            "case insensitive",
			message:         "SHIPPING TO AUSTRALIA?",
			expectedResult:  true,
			expectedCountry: "Australia",
		},
		{
			name:            "ship to Israel",
			message:         "Do you ship to Israel?",
			expectedResult:  true,
			expectedCountry: "Israel",
		},
		{
			name:            "ship to Thailand",
			message:         "Can I get shipping to Thailand?",
			expectedResult:  true,
			expectedCountry: "Thailand",
		},
		{
			name:            "ship to Germany",
			message:         "Shipping to Germany available?",
			expectedResult:  true,
			expectedCountry: "Germany",
		},
		{
			name:            "ship to South Korea",
			message:         "Can you ship to Korea?",
			expectedResult:  true,
			expectedCountry: "Korea",
		},
		{
			name:            "mixed case country name",
			message:         "ship to new zealand please",
			expectedResult:  true,
			expectedCountry: "New Zealand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isShipping, country := IsShippingInquiry(tt.message)
			assert.Equal(t, tt.expectedResult, isShipping)
			assert.Equal(t, tt.expectedCountry, country)
		})
	}
}

func TestGetShippingResponse(t *testing.T) {
	tests := []struct {
		name          string
		country       string
		checkResponse func(t *testing.T, response string)
	}{
		{
			name:    "response includes country name",
			country: "Ecuador",
			checkResponse: func(t *testing.T, response string) {
				assert.Contains(t, response, "Ecuador")
				assert.Contains(t, response, "shipping")
				assert.Contains(t, response, "14-21 business days")
				assert.Contains(t, response, "5-10 business days")
			},
		},
		{
			name:    "response includes your country placeholder",
			country: "your country",
			checkResponse: func(t *testing.T, response string) {
				assert.Contains(t, response, "your country")
				assert.Contains(t, response, "import taxes")
				assert.Contains(t, response, "customs duties")
			},
		},
		{
			name:    "response includes shipping policy URL",
			country: "USA",
			checkResponse: func(t *testing.T, response string) {
				assert.Contains(t, response, "USA")
				assert.Contains(t, response, "https://israeldefensestore.com/shipping-policy")
			},
		},
		{
			name:    "response warns about customs",
			country: "Canada",
			checkResponse: func(t *testing.T, response string) {
				assert.Contains(t, response, "Canada")
				assert.Contains(t, response, "import taxes")
				assert.Contains(t, response, "customs duties")
				assert.Contains(t, response, "checking with your local customs office")
			},
		},
		{
			name:    "response includes both shipping options",
			country: "Germany",
			checkResponse: func(t *testing.T, response string) {
				assert.Contains(t, response, "Germany")
				assert.Contains(t, response, "Standard Shipping")
				assert.Contains(t, response, "Express Shipping (EMS)")
			},
		},
		{
			name:    "empty country name",
			country: "",
			checkResponse: func(t *testing.T, response string) {
				// Should still contain the template text
				assert.Contains(t, response, "shipping")
				assert.Contains(t, response, "14-21 business days")
			},
		},
		{
			name:    "special characters in country",
			country: "Côte d'Ivoire",
			checkResponse: func(t *testing.T, response string) {
				assert.Contains(t, response, "Côte d'Ivoire")
				assert.Contains(t, response, "shipping")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := GetShippingResponse(tt.country)
			assert.NotEmpty(t, response)
			tt.checkResponse(t, response)
		})
	}
}

func TestShippingPolicyResponse_Contains_Required_Information(t *testing.T) {
	// Verify the constant contains all required information
	assert.Contains(t, ShippingPolicyResponse, "[COUNTRY]")
	assert.Contains(t, ShippingPolicyResponse, "import taxes")
	assert.Contains(t, ShippingPolicyResponse, "customs duties")
	assert.Contains(t, ShippingPolicyResponse, "Standard Shipping")
	assert.Contains(t, ShippingPolicyResponse, "Express Shipping (EMS)")
	assert.Contains(t, ShippingPolicyResponse, "14-21 business days")
	assert.Contains(t, ShippingPolicyResponse, "5-10 business days")
	assert.Contains(t, ShippingPolicyResponse, "https://israeldefensestore.com/shipping-policy")
}

func TestIsShippingInquiry_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedResult  bool
		expectedCountry string
	}{
		{
			name:            "very long message with shipping",
			message:         "I'm looking for a holster for my Glock 19 and I'm wondering if you can ship to Brazil because I live there and need it urgently",
			expectedResult:  true,
			expectedCountry: "Brazil",
		},
		{
			name:            "shipping keyword but different context",
			message:         "What's the connection between gun and holster?", // No shipping keywords
			expectedResult:  false,
			expectedCountry: "",
		},
		{
			name:            "unicode characters",
			message:         "שלום, האם אתם שולחים לישראל?", // Hebrew: "Hello, do you ship to Israel?"
			expectedResult:  false,                          // Won't detect Hebrew shipping keywords
			expectedCountry: "",
		},
		{
			name:            "multiple shipping keywords",
			message:         "Do you ship and deliver to Australia quickly?",
			expectedResult:  true,
			expectedCountry: "Australia",
		},
		{
			name:            "whitespace only",
			message:         "   \t\n  ",
			expectedResult:  false,
			expectedCountry: "",
		},
		{
			name:            "country name at end",
			message:         "shipping to Thailand",
			expectedResult:  true,
			expectedCountry: "Thailand",
		},
		{
			name:            "country name at beginning",
			message:         "Thailand shipping available?",
			expectedResult:  true,
			expectedCountry: "Thailand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isShipping, country := IsShippingInquiry(tt.message)
			assert.Equal(t, tt.expectedResult, isShipping)
			assert.Equal(t, tt.expectedCountry, country)
		})
	}
}

func TestGetShippingResponse_Concurrency(t *testing.T) {
	// Test that GetShippingResponse is safe for concurrent use
	countries := []string{"USA", "Canada", "UK", "Germany", "Japan", "Australia", "Brazil", "India", "France", "Italy"}
	done := make(chan bool, len(countries))

	for _, country := range countries {
		go func(c string) {
			response := GetShippingResponse(c)
			assert.NotEmpty(t, response)
			assert.Contains(t, response, c)
			done <- true
		}(country)
	}

	// Wait for all goroutines
	for i := 0; i < len(countries); i++ {
		<-done
	}
}

func TestIsShippingInquiry_AllSupportedCountries(t *testing.T) {
	// Test a selection of supported countries
	supportedCountries := []string{
		"ecuador", "usa", "united states", "uk", "united kingdom", "canada",
		"australia", "germany", "france", "italy", "spain", "brazil",
		"thailand", "japan", "india", "israel", "singapore", "new zealand",
	}

	for _, country := range supportedCountries {
		t.Run(country, func(t *testing.T) {
			message := "Can you ship to " + country + "?"
			isShipping, detectedCountry := IsShippingInquiry(message)
			assert.True(t, isShipping, "Should detect shipping inquiry for "+country)
			assert.NotEqual(t, "your country", detectedCountry, "Should detect specific country: "+country)
		})
	}
}
