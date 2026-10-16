package leads

import (
	"strings"
	"testing"
)

func TestLeadValidation(t *testing.T) {
	req := Request{Name: " Pemilik demo ", Phone: "0812-3456-7890", BusinessName: "Toko demo", BusinessType: "retail", City: "Jakarta", Consent: true, ConsentVersion: ConsentVersion, PrivacyVersion: PrivacyVersion, SourcePage: "/?phone=secret"}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if req.Phone != "+6281234567890" || req.Name != "Pemilik demo" || req.SourcePage != "/" {
		t.Fatalf("normalisasi gagal: %q %q %q", req.Phone, req.Name, req.SourcePage)
	}
	for name, mutate := range map[string]func(*Request){
		"consent":  func(r *Request) { r.Consent = false },
		"version":  func(r *Request) { r.ConsentVersion = "old" },
		"country":  func(r *Request) { r.Phone = "+12025550123" },
		"email":    func(r *Request) { r.Email = "Nama <email@example.test>" },
		"notes":    func(r *Request) { r.Notes = strings.Repeat("a", 1001) },
		"business": func(r *Request) { r.BusinessType = "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			copy := req
			mutate(&copy)
			if copy.Validate() == nil {
				t.Fatal("input tidak valid diterima")
			}
		})
	}
}
