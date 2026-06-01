package code

import "testing"

func TestExtract(t *testing.T) {
	tests := []struct {
		name  string
		title string
		cover string
		want  string
	}{
		{"bracketed", "[DSAM-002] Amateur AV 143cm ● Cute Face, Small Pussy", "", "DSAM-002"},
		{"plain hyphen + CJK", "CME-003 ひな", "", "CME-003"},
		{"bracketed mimk", "[MIMK-270] Kiss-free Mating Practice With The...", "", "MIMK-270"},
		{"series name, no code", "SexFriend 227「NUKKE -ヌケ- ドロシー編」", "", ""},
		{"hyphen in words is ignored", "Live-action Version Ibuki Aoi", "", ""},
		{"filename fallback", "あるコスプレ", "https://cosplay.jav.pw/wp-content/uploads/2026/05/dsam002pl.jpg", "DSAM-002"},
		{"filename leading zero", "ひな", "https://cosplay.jav.pw/wp-content/uploads/2026/05/cme0003jp.jpg", "CME-003"},
		{"numeric doujin filename", "同人作品", "https://cosplay.jav.pw/wp-content/uploads/2026/05/4069480top.jpg", ""},
		{"title wins over filename", "[ABP-123] foo", "https://x/dsam002pl.jpg", "ABP-123"},
		{"lowercase title normalized", "abc-045 something", "", "ABC-045"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Extract(tt.title, tt.cover); got != tt.want {
				t.Errorf("Extract(%q, %q) = %q, want %q", tt.title, tt.cover, got, tt.want)
			}
		})
	}
}
