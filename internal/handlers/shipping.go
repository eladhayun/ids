package handlers

import (
	"strings"
)

// ShippingPolicyResponse is the canned response for shipping inquiries
const ShippingPolicyResponse = `We ship worldwide to most countries, including [COUNTRY]. 

**Shipping Information:**
- **Standard Shipping:** Usually takes 14-21 business days.
- **Express Shipping (EMS):** Usually takes 5-10 business days.

Please note that customs regulations vary by country, and you are responsible for knowing your local laws regarding the import of tactical gear.

For our full shipping policy, please visit: https://israeldefensestore.com/shipping-policy`

// IsShippingInquiry checks if the user message is asking about shipping
func IsShippingInquiry(message string) (bool, string) {
	lowerMsg := strings.ToLower(message)

	// Keywords to detect shipping questions
	shippingKeywords := []string{"ship", "shipping", "delivery", "send to", "arrive"}

	isShipping := false
	for _, kw := range shippingKeywords {
		if strings.Contains(lowerMsg, kw) {
			isShipping = true
			break
		}
	}

	if !isShipping {
		return false, ""
	}

	// Extract country if present (simple heuristic)
	// This is a basic list, in a real app we might use a library or a longer list
	countries := []string{
		"ecuador", "usa", "united states", "uk", "united kingdom", "canada", "australia",
		"germany", "france", "italy", "spain", "brazil", "argentina", "chile", "mexico",
		"thailand", "philippines", "japan", "korea", "india",
	}

	detectedCountry := "your country"
	for _, country := range countries {
		if strings.Contains(lowerMsg, country) {
			// Capitalize first letter for display
			if len(country) <= 3 {
				detectedCountry = strings.ToUpper(country)
			} else {
				detectedCountry = strings.Title(country)
			}
			break
		}
	}

	return true, detectedCountry
}

// GetShippingResponse returns the formatted shipping response
func GetShippingResponse(country string) string {
	return strings.Replace(ShippingPolicyResponse, "[COUNTRY]", country, 1)
}
