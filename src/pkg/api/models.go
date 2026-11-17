package api

import "encoding/json"

// Account describes the brand the copy is for. It does not choose the billed
// account; the API key does.
type Account struct {
	Name     string `json:"name"`
	Industry string `json:"industry,omitempty"`
	Country  string `json:"country,omitempty"`
	Region   string `json:"region,omitempty"`
	Language string `json:"language,omitempty"`
	ParentID int    `json:"parentId,omitempty"`
}

// Touchpoints maps a touchpoint key (text, headline, body...) to its copy.
type Touchpoints map[string]string

// ScoreRequest is the body of POST /v1/score.
type ScoreRequest struct {
	Channel        string        `json:"channel,omitempty"`
	DisplayChannel string        `json:"displayChannel,omitempty"`
	Account        Account       `json:"account"`
	Variants       []Touchpoints `json:"variants"`
}

// OptimizeRequest is the body of POST /v1/optimize.
type OptimizeRequest struct {
	Channel         string         `json:"channel,omitempty"`
	DisplayChannel  string         `json:"displayChannel,omitempty"`
	Account         Account        `json:"account"`
	Brand           *Brand         `json:"brand,omitempty"`
	Generation      *Generation    `json:"generation,omitempty"`
	Variant         Touchpoints    `json:"variant"`
	Context         string         `json:"context,omitempty"`
	AlwaysUse       []string       `json:"alwaysUse,omitempty"`
	CharacterLimits map[string]int `json:"characterLimits,omitempty"`
}

// Brand holds the generator's style guidelines, exclusions and limits. The
// guideline sections are free-form and are sent unchanged.
type Brand struct {
	StyleSetName                 string                     `json:"styleSetName,omitempty"`
	ExclusionList                []string                   `json:"exclusionList,omitempty"`
	StyleGuidelines              map[string]json.RawMessage `json:"styleGuidelines,omitempty"`
	StyleGuidelinesPerTouchpoint map[string]json.RawMessage `json:"styleGuidelinesPerTouchpoint,omitempty"`
	CharacterLimits              map[string]int             `json:"characterLimits,omitempty"`
}

// Generation holds the generation settings. ContentSeedingEnabled is always
// sent because the API requires it whenever generation is present.
type Generation struct {
	AssistantID           string `json:"assistantId,omitempty"`
	ContentSeedingEnabled *bool  `json:"contentSeedingEnabled"`
}

// PerformanceRequest is the body of POST /v1/performance.
type PerformanceRequest struct {
	Channel        string        `json:"channel,omitempty"`
	DisplayChannel string        `json:"displayChannel,omitempty"`
	Account        Account       `json:"account"`
	Variants       []Touchpoints `json:"variants"`
}

// UpliftRequest is the body of POST /v1/uplift.
type UpliftRequest struct {
	Channel            string      `json:"channel,omitempty"`
	DisplayChannel     string      `json:"displayChannel,omitempty"`
	Account            Account     `json:"account"`
	Variant            Touchpoints `json:"variant"`
	ResponseCategories []string    `json:"responseCategories,omitempty"`
}

// Job statuses.
const (
	JobRunning   = "running"
	JobCompleted = "completed"
	JobFailed    = "failed"
)

// Job is a score or optimize run.
type Job struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
	ExpiresAt string    `json:"expiresAt"`
	Result    *Result   `json:"result"`
	Error     *JobError `json:"error"`
}

// Done reports whether the job has finished, successfully or not.
func (j *Job) Done() bool { return j.Status == JobCompleted || j.Status == JobFailed }

// JobError says why a job failed.
type JobError struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	Categories []string `json:"categories"`
}

// Result is the latest snapshot of a job's output.
type Result struct {
	Complete             bool      `json:"complete"`
	Brand                string    `json:"brand"`
	Industry             *string   `json:"industry"`
	Channel              string    `json:"channel"`
	DisplayChannel       *string   `json:"displayChannel"`
	ComplianceApplicable bool      `json:"complianceApplicable"`
	Variants             []Variant `json:"variants"`
	Control              *Variant  `json:"control"`
	Alternatives         []Variant `json:"alternatives"`
	Notice               *Notice   `json:"notice"`
}

// Variant is one scored piece of copy. A nil score is not available.
type Variant struct {
	Index           *int              `json:"index"`
	Rank            *int              `json:"rank"`
	Touchpoints     map[string]string `json:"touchpoints"`
	Text            string            `json:"text"`
	Performance     *int              `json:"performance"`
	PerformanceBand *string           `json:"performanceBand"`
	BrandAlignment  *int              `json:"brandAlignment"`
	BrandRationale  *string           `json:"brandRationale"`
	Compliance      *Compliance       `json:"compliance"`
	Emotions        []string          `json:"emotions"`
	Descriptors     []string          `json:"descriptors"`
}

// Compliance is a variant's compliance check.
type Compliance struct {
	Risk    string  `json:"risk"`
	State   string  `json:"state"`
	Summary *string `json:"summary"`
	Issues  []struct {
		Risk        *string `json:"risk"`
		Description *string `json:"description"`
		Regulation  *string `json:"regulation"`
	} `json:"issues"`
}

// Notice says why a run ended as it did.
type Notice struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PerformanceResponse is the answer of POST /v1/performance.
type PerformanceResponse struct {
	Model            string              `json:"model"`
	ResolvedLanguage string              `json:"resolvedLanguage"`
	LocaleSource     string              `json:"localeSource"`
	ClientModel      bool                `json:"clientModel"`
	Results          []PerformanceResult `json:"results"`
}

// PerformanceResult is the prediction for one variant.
type PerformanceResult struct {
	Text        string   `json:"text"`
	Performance *float64 `json:"performance"`
	Model       string   `json:"model"`
}

// UpliftResponse is the answer of POST /v1/uplift.
type UpliftResponse struct {
	Performance     *float64     `json:"performance"`
	Potential       *float64     `json:"potential"`
	AtPotential     bool         `json:"atPotential"`
	Resolution      *string      `json:"resolution"`
	ChannelCategory *string      `json:"channelCategory"`
	Vertical        *string      `json:"vertical"`
	Uplift          []UpliftItem `json:"uplift"`
	Notice          *Notice      `json:"notice"`
}

// UpliftItem is the projected uplift for one response category, in percent.
type UpliftItem struct {
	Category            string   `json:"category"`
	VsIndustryToday     *float64 `json:"vsIndustryToday"`
	VsIndustryProjected *float64 `json:"vsIndustryProjected"`
	Total               *float64 `json:"total"`
	Unavailable         *string  `json:"unavailable"`
}

// KeyLabel is a catalogue entry.
type KeyLabel struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Catalogue lists the values the run endpoints accept.
type Catalogue struct {
	DefaultChannel string `json:"defaultChannel"`
	Channels       []struct {
		Key         string     `json:"key"`
		Label       string     `json:"label"`
		Touchpoints []KeyLabel `json:"touchpoints"`
	} `json:"channels"`
	DisplayChannels []KeyLabel `json:"displayChannels"`
	Industries      []KeyLabel `json:"industries"`
	Limits          struct {
		MaxBodyBytes        int `json:"maxBodyBytes"`
		MaxTouchpointLength int `json:"maxTouchpointLength"`
		MaxScoreVariants    int `json:"maxScoreVariants"`
		MaxWaitSeconds      int `json:"maxWaitSeconds"`
	} `json:"limits"`
}

// VersionInfo is the answer of GET /version.
type VersionInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuildTime   string `json:"buildTime"`
	SpecDate    string `json:"specDate"`
	Environment string `json:"environment"`
}

// AssetMetadata is the metadata part of POST /v1/asset/create.
type AssetMetadata struct {
	Filename      string            `json:"filename"`
	MimeType      string            `json:"mimeType"`
	Length        int64             `json:"length"`
	ExpiresInDays int               `json:"expiresInDays,omitempty"`
	Owner         string            `json:"owner,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// Asset is one stored file.
type Asset struct {
	ID         string            `json:"id"`
	Filename   string            `json:"filename"`
	MimeType   string            `json:"mimeType"`
	Size       int64             `json:"size"`
	Version    int               `json:"version"`
	CreatedAt  string            `json:"createdAt"`
	ExpiresAt  string            `json:"expiresAt"`
	Attributes map[string]string `json:"attributes"`
}

// AssetList is one page of GET /v1/asset/list.
type AssetList struct {
	Assets     []Asset `json:"assets"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

// AssetURL is the answer of GET /v1/asset/retrieve without streaming.
type AssetURL struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	URLExpiresAt string `json:"urlExpiresAt"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mimeType"`
	Size         int64  `json:"size"`
	Version      int    `json:"version"`
	ExpiresAt    string `json:"expiresAt"`
}
