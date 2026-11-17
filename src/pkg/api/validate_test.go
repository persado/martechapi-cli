package api

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// validationFields returns the field of each ValidationError entry, or nil.
func validationFields(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		return nil
	}
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error %v is not a ValidationError", err)
	}
	fields := make([]string, len(ve))
	for i, d := range ve {
		fields[i] = d.Field
	}
	return fields
}

func assertFields(t *testing.T, err error, want []string) {
	t.Helper()
	if got := validationFields(t, err); !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v (error: %v)", got, want, err)
	}
}

func TestScoreRequestValidate(t *testing.T) {
	ok := func() ScoreRequest {
		return ScoreRequest{Account: Account{Name: "Acme"}, Variants: []Touchpoints{{"text": "Hi"}}}
	}
	tests := []struct {
		name string
		edit func(*ScoreRequest)
		want []string
	}{
		{"minimal", func(*ScoreRequest) {}, nil},
		{"channel and display channel", func(r *ScoreRequest) {
			r.Channel, r.DisplayChannel = "EMAIL_SUBJECT_BODY", "EMAIL"
			r.Variants = []Touchpoints{{"subject_line": "a", "body": "b"}}
		}, nil},
		{"unknown channel", func(r *ScoreRequest) { r.Channel = "BOGUS" }, []string{"channel"}},
		{"lower-case channel", func(r *ScoreRequest) { r.Channel = "email_subject" }, []string{"channel"}},
		{"unknown display channel", func(r *ScoreRequest) { r.DisplayChannel = "BOGUS" }, []string{"displayChannel"}},
		{"empty account name", func(r *ScoreRequest) { r.Account.Name = "" }, []string{"account.name"}},
		{"blank account name", func(r *ScoreRequest) { r.Account.Name = "  " }, []string{"account.name"}},
		{"account name 256 runes", func(r *ScoreRequest) { r.Account.Name = strings.Repeat("é", 256) }, nil},
		{"account name 257 runes", func(r *ScoreRequest) { r.Account.Name = strings.Repeat("é", 257) }, []string{"account.name"}},
		{"industry OTHER", func(r *ScoreRequest) { r.Account.Industry = "OTHER" }, nil},
		{"unknown industry", func(r *ScoreRequest) { r.Account.Industry = "ZZ" }, []string{"account.industry"}},
		{"country US", func(r *ScoreRequest) { r.Account.Country = "US" }, nil},
		{"country lower-case", func(r *ScoreRequest) { r.Account.Country = "us" }, []string{"account.country"}},
		{"country three letters", func(r *ScoreRequest) { r.Account.Country = "USA" }, []string{"account.country"}},
		{"region 64 runes", func(r *ScoreRequest) { r.Account.Region = strings.Repeat("r", 64) }, nil},
		{"region 65 runes", func(r *ScoreRequest) { r.Account.Region = strings.Repeat("r", 65) }, []string{"account.region"}},
		{"blank region", func(r *ScoreRequest) { r.Account.Region = "  " }, []string{"account.region"}},
		{"language en", func(r *ScoreRequest) { r.Account.Language = "en" }, nil},
		{"language three letters", func(r *ScoreRequest) { r.Account.Language = "eng" }, nil},
		{"language with region", func(r *ScoreRequest) { r.Account.Language = "en-US" }, []string{"account.language"}},
		{"language upper-case", func(r *ScoreRequest) { r.Account.Language = "EN" }, []string{"account.language"}},
		{"parent id 0 is omitted", func(r *ScoreRequest) { r.Account.ParentID = 0 }, nil},
		{"parent id 3", func(r *ScoreRequest) { r.Account.ParentID = 3 }, nil},
		{"negative parent id", func(r *ScoreRequest) { r.Account.ParentID = -1 }, []string{"account.parentId"}},
		{"no variants", func(r *ScoreRequest) { r.Variants = nil }, []string{"variants"}},
		{"four variants", func(r *ScoreRequest) {
			r.Variants = []Touchpoints{{"text": "a"}, {"text": "b"}, {"text": "c"}, {"text": "d"}}
		}, nil},
		{"five variants", func(r *ScoreRequest) {
			r.Variants = []Touchpoints{{"text": "a"}, {"text": "b"}, {"text": "c"}, {"text": "d"}, {"text": "e"}}
		}, []string{"variants"}},
		{"empty variant", func(r *ScoreRequest) { r.Variants = []Touchpoints{{}} }, []string{"variants[0]"}},
		{"second variant invalid", func(r *ScoreRequest) {
			r.Variants = []Touchpoints{{"text": "a"}, {"text": ""}}
		}, []string{"variants[1].text"}},
		{"touchpoint over 65536 runes", func(r *ScoreRequest) {
			r.Variants = []Touchpoints{{"text": strings.Repeat("é", MaxTouchpointLength+1)}}
		}, []string{"variants[0].text"}},
		{"default channel accepts single any key", func(r *ScoreRequest) {
			r.Variants = []Touchpoints{{"headline": "x"}}
		}, nil},
		{"default channel rejects second key", func(r *ScoreRequest) {
			r.Variants = []Touchpoints{{"text": "a", "headline": "b"}}
		}, []string{"variants[0].headline"}},
		{"all errors reported in order", func(r *ScoreRequest) {
			r.Channel = "BOGUS"
			r.Account = Account{}
			r.Variants = nil
		}, []string{"channel", "account.name", "variants"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ok()
			tc.edit(&r)
			assertFields(t, r.Validate(), tc.want)
		})
	}
}

func TestPerformanceRequestValidate(t *testing.T) {
	ok := func() PerformanceRequest {
		return PerformanceRequest{Account: Account{Name: "Acme"}, Variants: []Touchpoints{{"text": "Hi"}}}
	}
	tests := []struct {
		name string
		edit func(*PerformanceRequest)
		want []string
	}{
		{"minimal", func(*PerformanceRequest) {}, nil},
		{"ten variants", func(r *PerformanceRequest) {
			r.Variants = make([]Touchpoints, MaxPerformanceVariants)
			for i := range r.Variants {
				r.Variants[i] = Touchpoints{"text": "a"}
			}
		}, nil},
		{"eleven variants", func(r *PerformanceRequest) {
			r.Variants = make([]Touchpoints, MaxPerformanceVariants+1)
			for i := range r.Variants {
				r.Variants[i] = Touchpoints{"text": "a"}
			}
		}, []string{"variants"}},
		{"no variants", func(r *PerformanceRequest) { r.Variants = nil }, []string{"variants"}},
		{"unknown channel key", func(r *PerformanceRequest) {
			r.Channel = "PUSH_NOTIFICATION"
			r.Variants = []Touchpoints{{"headline": "a", "text": "b"}}
		}, []string{"variants[0].text"}},
		{"bad channel and account", func(r *PerformanceRequest) {
			r.Channel = "BOGUS"
			r.Account = Account{}
		}, []string{"channel", "account.name"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ok()
			tc.edit(&r)
			assertFields(t, r.Validate(), tc.want)
		})
	}
}

func TestOptimizeRequestValidate(t *testing.T) {
	ok := func() OptimizeRequest {
		return OptimizeRequest{Account: Account{Name: "Acme"}, Variant: Touchpoints{"text": "Hi"}}
	}
	tests := []struct {
		name string
		edit func(*OptimizeRequest)
		want []string
	}{
		{"minimal", func(*OptimizeRequest) {}, nil},
		{"empty variant", func(r *OptimizeRequest) { r.Variant = Touchpoints{} }, []string{"variant"}},
		{"variant key not in channel", func(r *OptimizeRequest) {
			r.Variant = Touchpoints{"text": "a", "headline": "b"}
		}, []string{"variant.headline"}},
		{"context 30000 runes", func(r *OptimizeRequest) { r.Context = strings.Repeat("é", MaxContextLength) }, nil},
		{"context 30001 runes", func(r *OptimizeRequest) { r.Context = strings.Repeat("a", MaxContextLength+1) }, []string{"context"}},
		{"context whitespace is not counted", func(r *OptimizeRequest) {
			r.Context = "  " + strings.Repeat("a", MaxContextLength) + "  "
		}, nil},
		{"ten always-use terms", func(r *OptimizeRequest) {
			r.AlwaysUse = make([]string, MaxAlwaysUse)
			for i := range r.AlwaysUse {
				r.AlwaysUse[i] = "term"
			}
		}, nil},
		{"eleven always-use terms", func(r *OptimizeRequest) {
			r.AlwaysUse = make([]string, MaxAlwaysUse+1)
			for i := range r.AlwaysUse {
				r.AlwaysUse[i] = "term"
			}
		}, []string{"alwaysUse"}},
		{"eleven terms with first empty", func(r *OptimizeRequest) {
			r.AlwaysUse = make([]string, MaxAlwaysUse+1)
			for i := range r.AlwaysUse {
				r.AlwaysUse[i] = "term"
			}
			r.AlwaysUse[0] = ""
		}, []string{"alwaysUse", "alwaysUse[0]"}},
		{"empty always-use term", func(r *OptimizeRequest) { r.AlwaysUse = []string{""} }, []string{"alwaysUse[0]"}},
		{"blank always-use term", func(r *OptimizeRequest) { r.AlwaysUse = []string{"   "} }, []string{"alwaysUse[0]"}},
		{"always-use term 100 runes", func(r *OptimizeRequest) {
			r.AlwaysUse = []string{strings.Repeat("é", MaxAlwaysUseLength)}
		}, nil},
		{"always-use term 101 runes", func(r *OptimizeRequest) {
			r.AlwaysUse = []string{strings.Repeat("é", MaxAlwaysUseLength+1)}
		}, []string{"alwaysUse[0]"}},
		{"character limit", func(r *OptimizeRequest) { r.CharacterLimits = map[string]int{"text": 140} }, nil},
		{"character limits sorted", func(r *OptimizeRequest) {
			r.CharacterLimits = map[string]int{"b": 70000, "a": 0}
		}, []string{"characterLimits.a", "characterLimits.b"}},
		{"empty brand", func(r *OptimizeRequest) { r.Brand = &Brand{} }, nil},
		{"brand style set name", func(r *OptimizeRequest) { r.Brand = &Brand{StyleSetName: "Acme style"} }, nil},
		{"blank brand style set name", func(r *OptimizeRequest) { r.Brand = &Brand{StyleSetName: "   "} }, []string{"brand.styleSetName"}},
		{"brand style set name 257 runes", func(r *OptimizeRequest) {
			r.Brand = &Brand{StyleSetName: strings.Repeat("s", 257)}
		}, []string{"brand.styleSetName"}},
		{"brand exclusion list", func(r *OptimizeRequest) {
			r.Brand = &Brand{ExclusionList: []string{"", "  ", "ok"}}
		}, []string{"brand.exclusionList[0]", "brand.exclusionList[1]"}},
		{"brand character limit", func(r *OptimizeRequest) {
			r.Brand = &Brand{CharacterLimits: map[string]int{"x": 0}}
		}, []string{"brand.characterLimits.x"}},
		{"generation without assistant", func(r *OptimizeRequest) { t := true; r.Generation = &Generation{ContentSeedingEnabled: &t} }, nil},
		{"generation without seeding flag", func(r *OptimizeRequest) { r.Generation = &Generation{AssistantID: "a"} }, []string{"generation.contentSeedingEnabled"}},
		{"generation assistant 128 runes", func(r *OptimizeRequest) {
			f := false
			r.Generation = &Generation{AssistantID: strings.Repeat("a", 128), ContentSeedingEnabled: &f}
		}, nil},
		{"generation assistant 129 runes", func(r *OptimizeRequest) {
			f := false
			r.Generation = &Generation{AssistantID: strings.Repeat("a", 129), ContentSeedingEnabled: &f}
		}, []string{"generation.assistantId"}},
		{"generation blank assistant", func(r *OptimizeRequest) {
			f := false
			r.Generation = &Generation{AssistantID: "   ", ContentSeedingEnabled: &f}
		}, []string{"generation.assistantId"}},
		{"all errors reported in order", func(r *OptimizeRequest) {
			r.Channel = "BOGUS"
			r.Account = Account{}
			r.Variant = nil
		}, []string{"channel", "account.name", "variant"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ok()
			tc.edit(&r)
			assertFields(t, r.Validate(), tc.want)
		})
	}
}

func TestUpliftRequestValidate(t *testing.T) {
	ok := func() UpliftRequest {
		return UpliftRequest{Account: Account{Name: "Acme"}, Variant: Touchpoints{"text": "Hi"}}
	}
	tests := []struct {
		name string
		edit func(*UpliftRequest)
		want []string
	}{
		{"minimal", func(*UpliftRequest) {}, nil},
		{"one category", func(r *UpliftRequest) { r.ResponseCategories = []string{"Open"} }, nil},
		{"all categories", func(r *UpliftRequest) {
			r.ResponseCategories = []string{"Click", "Conversion", "Open"}
		}, nil},
		{"four categories", func(r *UpliftRequest) {
			r.ResponseCategories = []string{"Click", "Conversion", "Open", "Click"}
		}, []string{"responseCategories", "responseCategories[3]"}},
		{"unknown category", func(r *UpliftRequest) { r.ResponseCategories = []string{"Click", "Bogus"} }, []string{"responseCategories[1]"}},
		{"lower-case category", func(r *UpliftRequest) { r.ResponseCategories = []string{"click"} }, []string{"responseCategories[0]"}},
		{"empty category", func(r *UpliftRequest) { r.ResponseCategories = []string{""} }, []string{"responseCategories[0]"}},
		{"duplicate category", func(r *UpliftRequest) { r.ResponseCategories = []string{"Open", "Open"} }, []string{"responseCategories[1]"}},
		{"empty variant", func(r *UpliftRequest) { r.Variant = nil }, []string{"variant"}},
		{"variant key not in channel", func(r *UpliftRequest) {
			r.Channel = "EMAIL_SUBJECT_BODY"
			r.Variant = Touchpoints{"text": "a"}
		}, []string{"variant.text"}},
		{"bad channel and account", func(r *UpliftRequest) {
			r.Channel = "BOGUS"
			r.Account = Account{}
		}, []string{"channel", "account.name"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ok()
			tc.edit(&r)
			assertFields(t, r.Validate(), tc.want)
		})
	}
}

func TestAssetMetadataValidate(t *testing.T) {
	ok := func() AssetMetadata {
		return AssetMetadata{Filename: "a.pdf", MimeType: "application/pdf", Length: 10}
	}
	tests := []struct {
		name     string
		maxBytes int64
		edit     func(*AssetMetadata)
		want     []string
	}{
		{"minimal", 0, func(*AssetMetadata) {}, nil},
		{"filename 255 runes", 0, func(m *AssetMetadata) { m.Filename = strings.Repeat("é", 255) }, nil},
		{"filename 256 runes", 0, func(m *AssetMetadata) { m.Filename = strings.Repeat("é", 256) }, []string{"filename"}},
		{"empty filename", 0, func(m *AssetMetadata) { m.Filename = "" }, []string{"filename"}},
		{"blank filename", 0, func(m *AssetMetadata) { m.Filename = "   " }, []string{"filename"}},
		{"tab in filename", 0, func(m *AssetMetadata) { m.Filename = "a\tb.pdf" }, []string{"filename"}},
		{"DEL in filename", 0, func(m *AssetMetadata) { m.Filename = "a\x7fb.pdf" }, []string{"filename"}},
		{"empty mime type", 0, func(m *AssetMetadata) { m.MimeType = "" }, []string{"mimeType"}},
		{"blank mime type", 0, func(m *AssetMetadata) { m.MimeType = "  " }, []string{"mimeType"}},
		{"mime type without slash", 0, func(m *AssetMetadata) { m.MimeType = "pdf" }, []string{"mimeType"}},
		{"mime type slash only", 0, func(m *AssetMetadata) { m.MimeType = "/" }, []string{"mimeType"}},
		{"mime type empty subtype", 0, func(m *AssetMetadata) { m.MimeType = "text/" }, []string{"mimeType"}},
		{"mime type with newline", 0, func(m *AssetMetadata) { m.MimeType = "application/pdf\n" }, []string{"mimeType"}},
		{"zero length", 0, func(m *AssetMetadata) { m.Length = 0 }, []string{"length"}},
		{"negative length", 0, func(m *AssetMetadata) { m.Length = -1 }, []string{"length"}},
		{"length at limit", 100, func(m *AssetMetadata) { m.Length = 100 }, nil},
		{"length over limit", 100, func(m *AssetMetadata) { m.Length = 101 }, []string{"length"}},
		{"no size limit", 0, func(m *AssetMetadata) { m.Length = 1 << 40 }, nil},
		{"expiry 0 is default", 0, func(m *AssetMetadata) { m.ExpiresInDays = 0 }, nil},
		{"expiry 1", 0, func(m *AssetMetadata) { m.ExpiresInDays = 1 }, nil},
		{"expiry 90", 0, func(m *AssetMetadata) { m.ExpiresInDays = 90 }, nil},
		{"expiry 91", 0, func(m *AssetMetadata) { m.ExpiresInDays = 91 }, []string{"expiresInDays"}},
		{"negative expiry", 0, func(m *AssetMetadata) { m.ExpiresInDays = -1 }, []string{"expiresInDays"}},
		{"empty owner", 0, func(m *AssetMetadata) { m.Owner = "" }, nil},
		{"owner e-mail", 0, func(m *AssetMetadata) { m.Owner = "user@example.com" }, nil},
		{"owner without at sign", 0, func(m *AssetMetadata) { m.Owner = "not-an-email" }, []string{"owner"}},
		{"attribute value 256 runes", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{"k": strings.Repeat("é", 256)}
		}, nil},
		{"attribute value 257 runes", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{"k": strings.Repeat("é", 257)}
		}, []string{"attributes.k"}},
		{"attribute key with allowed punctuation", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{"a.b_c-d": "v"}
		}, nil},
		{"attribute key 128 chars", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{strings.Repeat("a", 128): "v"}
		}, nil},
		{"attribute key 129 chars", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{strings.Repeat("a", 129): "v"}
		}, []string{"attributes." + strings.Repeat("a", 129)}},
		{"attribute key with space", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{"bad key": "v"}
		}, []string{"attributes.bad key"}},
		{"empty attribute key", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{"": "v"}
		}, []string{"attributes."}},
		{"twenty attributes", 0, func(m *AssetMetadata) {
			m.Attributes = attributeKeys(MaxAssetAttributes)
		}, nil},
		{"twenty-one attributes", 0, func(m *AssetMetadata) {
			m.Attributes = attributeKeys(MaxAssetAttributes + 1)
		}, []string{"attributes"}},
		{"attributes sorted", 0, func(m *AssetMetadata) {
			m.Attributes = map[string]string{"zz": strings.Repeat("v", 257), "aa": strings.Repeat("v", 257)}
		}, []string{"attributes.aa", "attributes.zz"}},
		{"all errors reported in order", 0, func(m *AssetMetadata) {
			*m = AssetMetadata{}
		}, []string{"filename", "mimeType", "length"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := ok()
			tc.edit(&m)
			assertFields(t, m.Validate(tc.maxBytes), tc.want)
		})
	}
}

func TestAssetMetadataValidateReservedKeys(t *testing.T) {
	for _, k := range reservedKey {
		t.Run(k, func(t *testing.T) {
			m := AssetMetadata{Filename: "a", MimeType: "a/b", Length: 1, Attributes: map[string]string{k: "v"}}
			assertFields(t, m.Validate(0), []string{"attributes." + k})
		})
	}
}

// attributeKeys returns n distinct, valid attribute keys.
func attributeKeys(n int) map[string]string {
	m := make(map[string]string, n)
	for i := 1; i <= n; i++ {
		m[strings.Repeat("k", i)] = "v"
	}
	return m
}

func TestTouchpointRules(t *testing.T) {
	tests := []struct {
		name    string
		channel string
		tp      Touchpoints
		want    []string
	}{
		{"nil touchpoints", "EMAIL_SUBJECT", nil, []string{"v"}},
		{"empty touchpoints", "EMAIL_SUBJECT", Touchpoints{}, []string{"v"}},
		{"email subject text", "EMAIL_SUBJECT", Touchpoints{"text": "x"}, nil},
		{"email subject any single key", "EMAIL_SUBJECT", Touchpoints{"anything": "x"}, nil},
		{"email subject upper-case single key", "EMAIL_SUBJECT", Touchpoints{"ANYTHING": "x"}, nil},
		{"email subject two keys", "EMAIL_SUBJECT", Touchpoints{"text": "x", "headline": "y"}, []string{"v.headline"}},
		{"email subject mixed-case key", "EMAIL_SUBJECT", Touchpoints{"Text": "x"}, nil},
		{"sms any single key", "SMS_EMS", Touchpoints{"whatever": "x"}, nil},
		{"sms two keys", "SMS_EMS", Touchpoints{"text": "x", "body": "y"}, []string{"v.body"}},
		{"push single unknown key", "PUSH_NOTIFICATION", Touchpoints{"text": "x"}, []string{"v.text"}},
		{"push mixed-case keys", "PUSH_NOTIFICATION", Touchpoints{"HEADLINE": "x", "Body": "y"}, nil},
		{"email subject body all keys", "EMAIL_SUBJECT_BODY",
			Touchpoints{"subject_line": "a", "preheader": "b", "headline": "c", "subheadline": "d", "body": "e", "cta": "f"}, nil},
		{"email subject body single known key", "EMAIL_SUBJECT_BODY", Touchpoints{"subject_line": "a"}, nil},
		{"email subject body single unknown key", "EMAIL_SUBJECT_BODY", Touchpoints{"foo": "x"}, []string{"v.foo"}},
		{"direct mail known keys", "DIRECT_MAIL", Touchpoints{"envelope_front": "a", "letter_cta": "b"}, nil},
		{"unknown channel skips key check", "BOGUS", Touchpoints{"anything": "x"}, nil},
		{"empty channel means email subject", "", Touchpoints{"headline": "x"}, nil},
		{"empty channel rejects second key", "", Touchpoints{"text": "a", "headline": "b"}, []string{"v.headline"}},
		{"empty value", "EMAIL_SUBJECT", Touchpoints{"text": ""}, []string{"v.text"}},
		{"value 65536 runes", "EMAIL_SUBJECT", Touchpoints{"text": strings.Repeat("é", MaxTouchpointLength)}, nil},
		{"value 65537 runes", "EMAIL_SUBJECT", Touchpoints{"text": strings.Repeat("é", MaxTouchpointLength+1)}, []string{"v.text"}},
		{"keys reported in sorted order", "PUSH_NOTIFICATION", Touchpoints{"b": "x", "a": "y"}, []string{"v.a", "v.b"}},
		{"key and value both wrong", "PUSH_NOTIFICATION", Touchpoints{"a": ""}, []string{"v.a", "v.a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c checker
			c.touchpoints("v", tc.channel, tc.tp)
			assertFields(t, c.err(), tc.want)
		})
	}
}

func TestEveryChannelAcceptsItsOwnTouchpoints(t *testing.T) {
	for _, ch := range Channels {
		t.Run(ch, func(t *testing.T) {
			keys := ChannelTouchpoints[ch]
			if len(keys) == 0 {
				t.Fatalf("no touchpoints listed for %s", ch)
			}
			tp := Touchpoints{}
			for _, k := range keys {
				tp[k] = "copy"
			}
			r := ScoreRequest{Channel: ch, Account: Account{Name: "Acme"}, Variants: []Touchpoints{tp}}
			assertFields(t, r.Validate(), nil)
		})
	}
}

func TestEveryDisplayChannelIsAccepted(t *testing.T) {
	for _, dc := range DisplayChannels {
		t.Run(dc, func(t *testing.T) {
			r := ScoreRequest{DisplayChannel: dc, Account: Account{Name: "Acme"}, Variants: []Touchpoints{{"text": "a"}}}
			assertFields(t, r.Validate(), nil)
		})
	}
}

func TestCheckerLength(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		minLen, maxLen int
		want           []string
	}{
		{"required when empty", "", 1, 5, []string{"f"}},
		{"required when blank", "   ", 1, 5, []string{"f"}},
		{"within bounds", "abc", 1, 5, nil},
		{"trimmed before counting", "  ab  ", 1, 2, nil},
		{"counts runes not bytes", "日本語", 1, 3, nil},
		{"too many runes", "日本語", 1, 2, []string{"f"}},
		{"below non-1 minimum", "a", 2, 5, []string{"f"}},
		{"above maximum", "abcdef", 1, 5, []string{"f"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c checker
			c.length("f", tc.value, tc.minLen, tc.maxLen)
			assertFields(t, c.err(), tc.want)
		})
	}
}

func TestCheckerLengthMessages(t *testing.T) {
	var c checker
	c.length("f", "", 1, 5)
	c.length("g", "abcdef", 2, 5)
	want := "f: is required\ng: must be 2 to 5 characters (got 6)"
	if got := c.err().Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestCheckerOneOf(t *testing.T) {
	allowed := []string{"A", "B"}
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"empty is skipped", "", nil},
		{"listed value", "B", nil},
		{"unlisted value", "C", []string{"f"}},
		{"case differs", "a", []string{"f"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c checker
			c.oneOf("f", tc.value, allowed)
			assertFields(t, c.err(), tc.want)
		})
	}
	var c checker
	c.oneOf("f", "C", allowed)
	if got, want := c.err().Error(), `f: "C" is not one of A, B`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestCheckerLimits(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]int
		want []string
	}{
		{"nil map", nil, nil},
		{"bounds accepted", map[string]int{"a": 1, "b": MaxTouchpointLength}, nil},
		{"zero rejected", map[string]int{"a": 0}, []string{"f.a"}},
		{"above maximum rejected", map[string]int{"a": MaxTouchpointLength + 1}, []string{"f.a"}},
		{"sorted keys", map[string]int{"z": 0, "a": 0}, []string{"f.a", "f.z"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c checker
			c.limits("f", tc.m)
			assertFields(t, c.err(), tc.want)
		})
	}
}

func TestValidationErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{"empty", nil, ""},
		{"one entry", ValidationError{{Field: "a", Reason: "is required"}}, "a: is required"},
		{"two entries", ValidationError{{Field: "a", Reason: "x"}, {Field: "b", Reason: "y"}}, "a: x\nb: y"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateReturnsValidationError(t *testing.T) {
	r := ScoreRequest{}
	err := r.Validate()
	var ve ValidationError
	if !errors.As(err, &ve) || len(ve) != 2 {
		t.Fatalf("Validate() = %#v, want a ValidationError with 2 entries", err)
	}
}

func TestIsControl(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'\x00', true},
		{'\n', true},
		{'\x1f', true},
		{' ', false},
		{'\x7f', true},
		{'é', false},
	}
	for _, tc := range tests {
		if got := isControl(tc.r); got != tc.want {
			t.Errorf("isControl(%U) = %v, want %v", tc.r, got, tc.want)
		}
	}
}

func TestValidJobID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid", "job_0123456789abcdef0123456789abcdef", true},
		{"upper-case hex", "job_0123456789ABCDEF0123456789ABCDEF", false},
		{"31 digits", "job_0123456789abcdef0123456789abcde", false},
		{"33 digits", "job_0123456789abcdef0123456789abcdef0", false},
		{"non-hex digit", "job_g123456789abcdef0123456789abcdef", false},
		{"wrong prefix", "jobs_0123456789abcdef0123456789abcdef", false},
		{"upper-case prefix", "JOB_0123456789abcdef0123456789abcdef", false},
		{"trailing newline", "job_0123456789abcdef0123456789abcdef\n", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidJobID(tc.id); got != tc.want {
				t.Errorf("ValidJobID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

func TestValidAssetID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"lower-case uuid", "123e4567-e89b-12d3-a456-426614174000", true},
		{"upper-case uuid", "123E4567-E89B-12D3-A456-426614174000", true},
		{"no dashes", "123e4567e89b12d3a456426614174000", false},
		{"wrong grouping", "123e456-7e89b-12d3-a456-426614174000", false},
		{"non-hex digit", "123e4567-e89b-12d3-a456-42661417400g", false},
		{"braces", "{123e4567-e89b-12d3-a456-426614174000}", false},
		{"surrounding space", " 123e4567-e89b-12d3-a456-426614174000", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidAssetID(tc.id); got != tc.want {
				t.Errorf("ValidAssetID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
