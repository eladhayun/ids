package handlers

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ShippingPolicyResponse is the canned response for shipping inquiries
const ShippingPolicyResponse = `Hi,

Thank you for your message and for your interest in our products.

Yes, we can ship to [COUNTRY]. However, as outlined in our **Shipping Policy**, international orders may be subject to **import taxes, customs duties, and/or handling fees**, which are determined by your country's customs authorities. These charges are **not included** in the price of the item or the shipping cost, and are the **sole responsibility of the recipient**.

Unfortunately, we do not have control over these charges and cannot predict their exact amount, as they vary from country to country. We always recommend **checking with your local customs office before placing an order** — especially for items like firearm conversion kits, which may be subject to additional scrutiny or regulation.

**Shipping Times:**
- **Standard Shipping:** Usually takes 14-21 business days
- **Express Shipping (EMS):** Usually takes 5-10 business days

If you have any other questions or need help placing your order, we're happy to assist.

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
		"thailand", "philippines", "japan", "korea", "south korea", "india", "china",
		"israel", "netherlands", "belgium", "sweden", "norway", "denmark", "finland",
		"poland", "portugal", "greece", "turkey", "switzerland", "austria", "ireland",
		"new zealand", "singapore", "malaysia", "indonesia", "vietnam", "taiwan",
		"hong kong", "uae", "saudi arabia", "south africa", "egypt", "peru", "colombia",
	}

	detectedCountry := "your country"
	caser := cases.Title(language.English)
	for _, country := range countries {
		if strings.Contains(lowerMsg, country) {
			// Capitalize first letter for display
			if len(country) <= 3 {
				detectedCountry = strings.ToUpper(country)
			} else {
				detectedCountry = caser.String(country)
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
