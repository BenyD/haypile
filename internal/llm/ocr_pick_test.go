package llm

import "testing"

func TestVisionModelID(t *testing.T) {
	cases := []struct {
		name string
		ids  []string
		want string
	}{
		{"empty", nil, ""},
		{"text only", []string{"llama3.2:3b", "qwen2.5:7b"}, ""},
		{"llava after text", []string{"llama3.2:3b", "llava:13b"}, "llava:13b"},
		{"dedicated ocr beats general vision", []string{"llava:13b", "Unlimited-OCR"}, "Unlimited-OCR"},
		{"qwen vl naming", []string{"llama3.2:3b", "qwen2.5-vl:7b"}, "qwen2.5-vl:7b"},
		{"llama vision naming", []string{"llama3.2-vision:11b"}, "llama3.2-vision:11b"},
		{"case insensitive", []string{"MiniCPM-V-2.6"}, "MiniCPM-V-2.6"},
	}
	for _, c := range cases {
		if got := VisionModelID(c.ids); got != c.want {
			t.Errorf("%s: VisionModelID(%v) = %q, want %q", c.name, c.ids, got, c.want)
		}
	}
}
