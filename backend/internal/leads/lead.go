package leads

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	errs "github.com/storepos/storepos/internal/platform/errors"
)

const ConsentVersion = "demo-2026-10-14"
const PrivacyVersion = "privacy-2026-10-14"
const ConsentText = "Saya setuju pengelola StorePOS menyimpan dan menggunakan data ini untuk menghubungi saya terkait permintaan demo, sesuai Kebijakan Privasi."

type Request struct {
	Name             string `json:"name"`
	Phone            string `json:"phone"`
	BusinessName     string `json:"business_name"`
	BusinessType     string `json:"business_type"`
	City             string `json:"city"`
	Email            string `json:"email"`
	Branches         string `json:"branches"`
	Terminals        string `json:"terminals"`
	CurrentSystem    string `json:"current_system"`
	Notes            string `json:"notes"`
	ContactTime      string `json:"contact_time"`
	Consent          bool   `json:"consent"`
	MarketingConsent bool   `json:"marketing_consent"`
	ConsentVersion   string `json:"consent_version"`
	PrivacyVersion   string `json:"privacy_version"`
	StartedAt        string `json:"started_at"`
	Website          string `json:"website"`
	SourcePage       string `json:"source_page"`
	SourceTab        string `json:"source_tab"`
}

var phonePattern = regexp.MustCompile(`^\+628\d{7,12}$`)
var phoneSeparators = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "(", "", ")", "", ".", "", "-", "")

func NormalizePhone(value string) string {
	value = phoneSeparators.Replace(strings.TrimSpace(value))
	if strings.HasPrefix(value, "08") {
		value = "+62" + value[1:]
	} else if strings.HasPrefix(value, "62") {
		value = "+" + value
	}
	if !phonePattern.MatchString(value) {
		return ""
	}
	return value
}

func (req *Request) Validate() error {
	var details []errs.FieldError
	invalid := func(field, message string) {
		details = append(details, errs.FieldError{Field: field, Code: "invalid", Message: message})
	}
	for _, field := range []struct {
		name  string
		value *string
		max   int
	}{{"name", &req.Name, 100}, {"business_name", &req.BusinessName, 120}, {"city", &req.City, 80}} {
		*field.value = strings.TrimSpace(*field.value)
		length := utf8.RuneCountInString(*field.value)
		if length < 2 || length > field.max {
			invalid(field.name, "Isi dengan panjang teks yang sesuai.")
		}
	}
	req.Phone = NormalizePhone(req.Phone)
	if req.Phone == "" {
		invalid("phone", "Gunakan nomor Indonesia yang diawali 08, 62, atau +62.")
	}
	if !oneOf(req.BusinessType, "retail", "restaurant", "pharmacy", "other") {
		invalid("business_type", "Pilih jenis usaha Anda.")
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email != "" {
		address, err := mail.ParseAddress(req.Email)
		domain := req.Email[strings.LastIndex(req.Email, "@")+1:]
		if err != nil || address.Address != req.Email || len(req.Email) > 254 || !strings.Contains(domain, ".") {
			invalid("email", "Masukkan alamat email yang valid.")
		}
	}
	req.Notes = strings.TrimSpace(req.Notes)
	if utf8.RuneCountInString(req.Notes) > 1000 {
		invalid("notes", "Catatan maksimal 1.000 karakter.")
	}
	for _, field := range []struct {
		name, value string
		options     []string
	}{
		{"branches", req.Branches, []string{"", "1", "2-5", "6+"}},
		{"terminals", req.Terminals, []string{"", "1", "2-3", "4+"}},
		{"current_system", req.CurrentSystem, []string{"", "manual", "spreadsheet", "app", "other"}},
		{"contact_time", req.ContactTime, []string{"", "morning", "afternoon", "evening"}},
	} {
		if !oneOf(field.value, field.options...) {
			invalid(field.name, "Pilih opsi yang tersedia.")
		}
	}
	if !req.Consent {
		invalid("consent", "Persetujuan diperlukan untuk menghubungi Anda terkait demo.")
	}
	if req.ConsentVersion != ConsentVersion || req.PrivacyVersion != PrivacyVersion {
		invalid("consent", "Versi persetujuan berubah. Muat ulang halaman dan periksa kembali.")
	}
	req.SourcePage = "/"
	if !oneOf(req.SourceTab, "", "retail", "restaurant", "pharmacy", "other") {
		req.SourceTab = ""
	}
	if len(details) > 0 {
		return errs.Validation("Periksa isian formulir demo.").WithDetails(details...)
	}
	return nil
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}
