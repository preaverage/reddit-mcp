package reddit

import "encoding/json"

var sensitiveMeFields = []string{
	"email",
	"linked_identities",
	"has_external_account",
	"oauth_client_id",
	"password_set",
	"force_password_reset",
	"has_stripe_subscription",
	"has_paypal_subscription",
	"has_android_subscription",
	"has_ios_subscription",
	"has_gold_subscription",
	"has_subscribed_to_premium",
	"coins",
	"gold_creddits",
	"gold_expiration",
	"features",
}

func redact(raw json.RawMessage, fields []string) json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return raw
	}

	for _, f := range fields {
		delete(obj, f)
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}

	return out
}
