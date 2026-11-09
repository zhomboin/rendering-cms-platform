package postbridge

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestImageSnapshotPolicy(t *testing.T) {
	const photo = "https://images.example/album/sample.png"
	const image = "![photo](" + photo + ")"
	cases := []struct {
		name, body string
		enabled    bool
		urls       []string
	}{
		{"duplicate image and ordinary link", image + "\n" + image + "\n[link](https://elsewhere.example/)", true, []string{photo}},
		{"image beside inline code and a link", "\x60code\x60 [link](https://elsewhere.example/) " + image, true, []string{photo}},
		{"image syntax within ordinary link destination", "[link](https://elsewhere.example/" + image + ")", true, nil},
		{"image nested in ordinary link label", "[" + image + "](https://elsewhere.example/)", true, nil},
		{"image nested in a multiline link label", "[text\n" + image + "\n](https://elsewhere.example/)", true, nil},
		{"escaped bracket before image", "\\[ " + image, true, []string{photo}},
		{"bracket in inline code before image", "\x60[\x60 " + image, true, []string{photo}},
		{"indented code", "    " + image, true, nil},
		{"plain target", image, false, nil},
		{"reference image", "![photo][ref]\n\n[ref]: " + photo, true, nil},
		{"HTML comment", "<!--\n" + image + "\n-->", true, nil},
		{"HTML image", "<img src=\"" + photo + "\">", true, nil},
		{"signed image", "![photo](" + photo + "?signature=test)", true, nil},
		{"fragment", "![photo](" + photo + "#image)", true, nil},
		{"userinfo", "![photo](https://images.example@elsewhere.example/sample.png)", true, nil},
		{"explicit port", "![photo](https://images.example:443/sample.png)", true, nil},
		{"relative", "![photo](/sample.png)", true, nil},
		{"parent path", "![photo](https://images.example/%2e%2e/sample.png)", true, nil},
		{"uppercase host", "![photo](https://IMAGES.example/sample.png)", true, nil},
		{"literal image", "\x60" + image + "\x60", true, nil},
		{"multiline literal image", "\x60code\n" + image + "\n\x60", true, nil},
		{"escaped image", "\\" + image, true, nil},
		{"ambiguous fence", "- list\n  \x60\x60\x60\n" + image + "\n  \x60\x60\x60", true, nil},
		{"too many occurrences", strings.Repeat(image+"\n", 21), true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := Snapshot{
				Title:      "A sample title",
				Body:       tc.body,
				BodyFormat: "markdown",
			}
			out, fingerprint, urls, err := validateSnapshot(in, tc.enabled)
			if tc.urls == nil {
				if !errors.Is(err, ErrUnsupportedContent) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(urls, tc.urls) || out.Body != in.Body || len(out.Assets) != 0 || len(fingerprint) != 64 {
				t.Fatalf("snapshot = %+v, URLs = %v, fingerprint = %q, error = %v", out, urls, fingerprint, err)
			}
		})
	}
}
