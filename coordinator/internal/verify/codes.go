package verify

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"strings"
)

var validChannels = map[string]bool{"sms": true, "call": true, "dtmf": true}
var validBindings = map[string]bool{"derive": true, "claim": true}

// channelNeedsCode reports whether a channel carries a code (SMS, DTMF) versus
// binding purely on caller ID (missed call).
func channelNeedsCode(channel string) bool {
	return channel == "sms" || channel == "dtmf"
}

// validateCombo enforces the channel/binding rules. Missed call (channel "call")
// carries no code, so the caller ID is the binder and only "claim" is valid.
func validateCombo(channel, binding string) error {
	if !validChannels[channel] {
		return &ValidationError{Field: "channel", Message: "must be one of: sms, call, dtmf"}
	}
	if !validBindings[binding] {
		return &ValidationError{Field: "binding_mode", Message: "must be one of: derive, claim"}
	}
	if channel == "call" && binding != "claim" {
		return &ValidationError{Field: "binding_mode", Message: "missed call (channel 'call') requires claim binding"}
	}
	return nil
}

// generateCode returns an n-digit numeric code using a cryptographic source.
func generateCode(n int) (string, error) {
	if n <= 0 {
		n = 6
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + d.Int64()))
	}
	return b.String(), nil
}

// extractCode reduces an inbound body to its digits and returns them when the
// length is plausible for a code. Handles "123456", "your code is 123456", and
// "123 456" alike; rejects noisy bodies by returning "".
func extractCode(body string) string {
	var b strings.Builder
	for _, r := range body {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) >= 4 && len(s) <= 12 {
		return s
	}
	return ""
}

// buildInstructions returns the human action text and a tap-to-act deep link.
func buildInstructions(channel, msisdn, code string) (action, deepLink string) {
	switch channel {
	case "sms":
		return fmt.Sprintf("Send %s to %s", code, msisdn), "sms:" + msisdn + "?body=" + url.QueryEscape(code)
	case "call":
		return fmt.Sprintf("Give a missed call to %s from your phone", msisdn), "tel:" + msisdn
	case "dtmf":
		return fmt.Sprintf("Call %s and enter %s on the keypad", msisdn, code), "tel:" + msisdn
	}
	return "", ""
}
