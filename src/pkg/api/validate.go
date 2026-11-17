package api

import (
	"fmt"
	"maps"
	"mime"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Channels are the values of channel, in catalogue order.
var Channels = []string{"EMAIL_SUBJECT", "EMAIL_SUBJECT_BODY", "SMS_EMS", "PUSH_NOTIFICATION", "IN_APP_BODY",
	"WEB_PAGE", "WEB_BANNER", "ADWORD", "FACEBOOK_AD", "LINKEDIN_AD", "DIRECT_MAIL"}

// DefaultChannel is the channel the API uses when a request names none.
const DefaultChannel = "EMAIL_SUBJECT"

// ChannelTouchpoints lists the touchpoint keys each channel accepts.
var ChannelTouchpoints = map[string][]string{
	"EMAIL_SUBJECT":      {"text"},
	"EMAIL_SUBJECT_BODY": {"subject_line", "preheader", "headline", "subheadline", "body", "cta"},
	"SMS_EMS":            {"text"},
	"PUSH_NOTIFICATION":  {"headline", "body"},
	"IN_APP_BODY":        {"header", "subheadline", "body", "cta"},
	"WEB_PAGE":           {"preheadline", "headline", "subheadline", "body", "cta"},
	"WEB_BANNER":         {"preheadline", "headline", "subheadline", "body", "cta"},
	"ADWORD":             {"headline", "description"},
	"FACEBOOK_AD":        {"text", "headline", "link_description"},
	"LINKEDIN_AD":        {"text", "headline", "link_description"},
	"DIRECT_MAIL": {"envelope_front", "envelope_back", "letter_preheadline", "letter_headline",
		"letter_subheadline", "letter_body", "letter_cta", "letter_closing"},
}

// DisplayChannels are the values of displayChannel.
var DisplayChannels = []string{"ADWORD", "APP_STORE_PAGE", "AUTOMATED_CALL_PROMPT", "BILLBOARD", "CALL_CENTER_SCRIPT",
	"CREATE_FROM_BRIEF", "CUSTOMER_SUPPORT_CHAT", "DESKTOP_NOTIFICATION", "DIRECT_MAIL", "DISPLAY_BANNER", "EMAIL",
	"EMAIL_BANNER", "EMAIL_BODY", "EMAIL_SUBJECT", "EMAIL_SUBJECT_BODY", "FACEBOOK_AD", "FLASH_SMS", "FLYER",
	"GMAIL_SPONSORED_PROMOTION", "INLINE_AD", "IN_APP_BODY", "JOURNEY", "LINKEDIN_AD", "MMS", "MOBILE_APP",
	"MULTI_CHANNEL_TEST", "OUT_OF_HOME_AD", "POS", "PUSH_NOTIFICATION", "PUSH_NOTIFICATION_IN_APP_BODY",
	"RADIO_SCRIPT", "SMS_EMS", "SNAPCHAT", "TEST_PREHEADER", "TIK_TOK", "WEB_BANNER", "WEB_PAGE", "WHATSAPP"}

// Industries are the values of account.industry.
var Industries = []string{"AUTOMOTIVE", "B2B_SERVICES", "CHARITIES_GOOD_WILL", "CONSUMER_GOODS_SERVICES", "DATING",
	"EDUCATION", "ENERGY", "FASHION_APPAREL", "FINANCIAL_SERVICES", "GAMING_GAMBLING", "HEALTHCARE", "INSURANCE",
	"INTERNET_CABLE", "MEDIA_ENTERTAINMENT", "OTHER", "RETAIL_ECOMMERCE", "TECHNOLOGY_APPS_SOFTWARE",
	"TELECOMMUNICATIONS", "TRAVEL_HOTELS"}

// ResponseCategories are the values of an uplift request's responseCategories.
var ResponseCategories = []string{"Click", "Conversion", "Open"}

// Limits from the API specification.
const (
	MaxScoreVariants       = 4
	MaxPerformanceVariants = 10
	MaxTouchpointLength    = 65536
	MaxContextLength       = 30000
	MaxAlwaysUse           = 10
	MaxAlwaysUseLength     = 100
	MaxWaitSeconds         = 50
	MaxAssetAttributes     = 20
	MaxAssetListLimit      = 200
	MaxExpiresInDays       = 90
)

var (
	countryRe   = regexp.MustCompile(`^[A-Z]{2}$`)
	languageRe  = regexp.MustCompile(`^[a-z]{2,3}$`)
	jobIDRe     = regexp.MustCompile(`^job_[0-9a-f]{32}$`)
	uuidRe      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	attrKeyRe   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	reservedKey = []string{"filename", "content-type", "owner", "objectId", "version", "contentLength", "expiresAt"}
)

// ValidationError lists every invalid field of a request.
type ValidationError []ErrorDetail

func (v ValidationError) Error() string {
	parts := make([]string, len(v))
	for i, d := range v {
		parts[i] = d.Field + ": " + d.Reason
	}
	return strings.Join(parts, "\n")
}

type checker struct{ errs ValidationError }

func (c *checker) add(field, format string, args ...any) {
	c.errs = append(c.errs, ErrorDetail{Field: field, Reason: fmt.Sprintf(format, args...)})
}

func (c *checker) err() error {
	if len(c.errs) == 0 {
		return nil
	}
	return c.errs
}

func (c *checker) oneOf(field, v string, allowed []string) {
	if v != "" && !slices.Contains(allowed, v) {
		c.add(field, "%q is not one of %s", v, strings.Join(allowed, ", "))
	}
}

func (c *checker) length(field, v string, minLen, maxLen int) {
	n := utf8.RuneCountInString(strings.TrimSpace(v))
	switch {
	case n < minLen && minLen == 1:
		c.add(field, "is required")
	case n < minLen || n > maxLen:
		c.add(field, "must be %d to %d characters (got %d)", minLen, maxLen, n)
	}
}

func (c *checker) channels(channel, display string) {
	c.oneOf("channel", channel, Channels)
	c.oneOf("displayChannel", display, DisplayChannels)
}

func (c *checker) account(a Account) {
	c.length("account.name", a.Name, 1, 256)
	c.oneOf("account.industry", a.Industry, Industries)
	if a.Country != "" && !countryRe.MatchString(a.Country) {
		c.add("account.country", "must be two capital letters, such as US or GR")
	}
	if a.Region != "" {
		c.length("account.region", a.Region, 1, 64)
	}
	if a.Language != "" && !languageRe.MatchString(a.Language) {
		c.add("account.language", "must be a lower-case language code without region, such as en or fr")
	}
	if a.ParentID < 0 {
		c.add("account.parentId", "must be at least 1")
	}
}

func (c *checker) touchpoints(field, channel string, t Touchpoints) {
	if len(t) == 0 {
		c.add(field, "needs at least one touchpoint, such as {\"text\": \"...\"}")
		return
	}
	if channel == "" {
		channel = DefaultChannel
	}
	allowed, known := ChannelTouchpoints[channel]
	anyKey := len(t) == 1 && (channel == "EMAIL_SUBJECT" || channel == "SMS_EMS")
	for _, k := range slices.Sorted(maps.Keys(t)) {
		key := strings.ToLower(strings.TrimSpace(k))
		if known && !anyKey && !slices.Contains(allowed, key) {
			c.add(field+"."+k, "is not a touchpoint of %s; use %s", channel, strings.Join(allowed, ", "))
		}
		if n := utf8.RuneCountInString(t[k]); n < 1 || n > MaxTouchpointLength {
			c.add(field+"."+k, "must be 1 to %d characters", MaxTouchpointLength)
		}
	}
}

func (c *checker) limits(field string, m map[string]int) {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if v := m[k]; v < 1 || v > MaxTouchpointLength {
			c.add(field+"."+k, "must be 1 to %d", MaxTouchpointLength)
		}
	}
}

// Validate checks r against the API's rules.
func (r *ScoreRequest) Validate() error {
	var c checker
	c.channels(r.Channel, r.DisplayChannel)
	c.account(r.Account)
	if n := len(r.Variants); n < 1 || n > MaxScoreVariants {
		c.add("variants", "must hold 1 to %d variants (got %d)", MaxScoreVariants, n)
	}
	for i, v := range r.Variants {
		c.touchpoints(fmt.Sprintf("variants[%d]", i), r.Channel, v)
	}
	return c.err()
}

// Validate checks r against the API's rules.
func (r *OptimizeRequest) Validate() error {
	var c checker
	c.channels(r.Channel, r.DisplayChannel)
	c.account(r.Account)
	c.touchpoints("variant", r.Channel, r.Variant)
	if n := utf8.RuneCountInString(strings.TrimSpace(r.Context)); n > MaxContextLength {
		c.add("context", "must be at most %d characters (got %d)", MaxContextLength, n)
	}
	if len(r.AlwaysUse) > MaxAlwaysUse {
		c.add("alwaysUse", "must hold at most %d terms (got %d)", MaxAlwaysUse, len(r.AlwaysUse))
	}
	for i, t := range r.AlwaysUse {
		c.length(fmt.Sprintf("alwaysUse[%d]", i), t, 1, MaxAlwaysUseLength)
	}
	c.limits("characterLimits", r.CharacterLimits)
	if b := r.Brand; b != nil {
		if b.StyleSetName != "" {
			c.length("brand.styleSetName", b.StyleSetName, 1, 256)
		}
		for i, e := range b.ExclusionList {
			if strings.TrimSpace(e) == "" {
				c.add(fmt.Sprintf("brand.exclusionList[%d]", i), "must not be empty")
			}
		}
		c.limits("brand.characterLimits", b.CharacterLimits)
	}
	if g := r.Generation; g != nil {
		if g.AssistantID != "" {
			c.length("generation.assistantId", g.AssistantID, 1, 128)
		}
		if g.ContentSeedingEnabled == nil {
			c.add("generation.contentSeedingEnabled", "is required when generation is set; use true or false (--content-seeding)")
		}
	}
	return c.err()
}

// Validate checks r against the API's rules.
func (r *PerformanceRequest) Validate() error {
	var c checker
	c.channels(r.Channel, r.DisplayChannel)
	c.account(r.Account)
	if n := len(r.Variants); n < 1 || n > MaxPerformanceVariants {
		c.add("variants", "must hold 1 to %d variants (got %d)", MaxPerformanceVariants, n)
	}
	for i, v := range r.Variants {
		c.touchpoints(fmt.Sprintf("variants[%d]", i), r.Channel, v)
	}
	return c.err()
}

// Validate checks r against the API's rules.
func (r *UpliftRequest) Validate() error {
	var c checker
	c.channels(r.Channel, r.DisplayChannel)
	c.account(r.Account)
	c.touchpoints("variant", r.Channel, r.Variant)
	if len(r.ResponseCategories) > len(ResponseCategories) {
		c.add("responseCategories", "must hold at most %d categories", len(ResponseCategories))
	}
	seen := map[string]bool{}
	for i, rc := range r.ResponseCategories {
		f := fmt.Sprintf("responseCategories[%d]", i)
		if !slices.Contains(ResponseCategories, rc) {
			c.add(f, "%q is not one of %s", rc, strings.Join(ResponseCategories, ", "))
		} else if seen[rc] {
			c.add(f, "%q is listed twice", rc)
		}
		seen[rc] = true
	}
	return c.err()
}

// Validate checks m against the API's rules. maxBytes is the server's file
// size limit; 0 skips that check.
func (m *AssetMetadata) Validate(maxBytes int64) error {
	var c checker
	c.length("filename", m.Filename, 1, 255)
	if strings.ContainsFunc(m.Filename, isControl) {
		c.add("filename", "must not contain control characters")
	}
	if strings.TrimSpace(m.MimeType) == "" {
		c.add("mimeType", "is required, such as application/pdf")
	} else if _, _, err := mime.ParseMediaType(m.MimeType); err != nil || !strings.Contains(m.MimeType, "/") || strings.ContainsFunc(m.MimeType, isControl) {
		c.add("mimeType", "%q is not a media type such as application/pdf", m.MimeType)
	}
	switch {
	case m.Length < 1:
		c.add("length", "must be at least 1 byte; empty files cannot be uploaded")
	case maxBytes > 0 && m.Length > maxBytes:
		c.add("length", "file is %d bytes; the limit is %d", m.Length, maxBytes)
	}
	if m.ExpiresInDays != 0 && (m.ExpiresInDays < 1 || m.ExpiresInDays > MaxExpiresInDays) {
		c.add("expiresInDays", "must be 1 to %d", MaxExpiresInDays)
	}
	if m.Owner != "" && !strings.Contains(m.Owner, "@") {
		c.add("owner", "must be an e-mail address")
	}
	if len(m.Attributes) > MaxAssetAttributes {
		c.add("attributes", "must hold at most %d keys (got %d)", MaxAssetAttributes, len(m.Attributes))
	}
	for _, k := range slices.Sorted(maps.Keys(m.Attributes)) {
		switch {
		case !attrKeyRe.MatchString(k):
			c.add("attributes."+k, "key must be 1 to 128 characters of A-Z, a-z, 0-9, '.', '_' or '-'")
		case slices.Contains(reservedKey, k):
			c.add("attributes."+k, "is reserved by the API")
		}
		if utf8.RuneCountInString(m.Attributes[k]) > 256 {
			c.add("attributes."+k, "value must be at most 256 characters")
		}
	}
	return c.err()
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// ValidJobID reports whether id has the job id format job_<32 hex digits>.
func ValidJobID(id string) bool { return jobIDRe.MatchString(id) }

// ValidAssetID reports whether id is a UUID.
func ValidAssetID(id string) bool { return uuidRe.MatchString(id) }
