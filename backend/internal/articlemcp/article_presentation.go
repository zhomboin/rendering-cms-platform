package articlemcp

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	errCover      = errors.New("封面须为长期有效的 HTTP(S) 图片地址或以 / 开头的站内路径，传空字符串可清除")
	errRank       = errors.New("精选排序须为 1 到 999 的整数")
	errEditFields = errors.New("请提交封面或精选设置，或同时提交完整标题和正文")
)

type Presentation struct {
	CoverImageURL *string `json:"coverImageUrl,omitempty" jsonschema:"长期有效的图片地址或站内路径；传空字符串清除封面"`
	Featured      *bool   `json:"featured,omitempty" jsonschema:"是否设为首页精选"`
	FeaturedRank  *int32  `json:"featuredRank,omitempty" jsonschema:"精选排序 1 到 999，越小越靠前，默认 100"`
}

func (p Presentation) validate() error {
	if p.FeaturedRank != nil && (*p.FeaturedRank < 1 || *p.FeaturedRank > 999) {
		return errRank
	}
	if p.CoverImageURL != nil && !validCoverURL(*p.CoverImageURL) {
		return errCover
	}
	return nil
}

func validCoverURL(raw string) bool {
	if raw == "" {
		return true
	}
	if strings.Contains(raw, `\`) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return false
	}
	if strings.HasPrefix(raw, "/") {
		if strings.HasPrefix(raw, "//") {
			return false
		}
	} else if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return false
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	for key := range query {
		key = strings.ToLower(key)
		if strings.HasPrefix(key, "x-amz-") || strings.HasPrefix(key, "x-goog-") {
			return false
		}
		switch key {
		case "awsaccesskeyid", "googleaccessid", "signature", "expires":
			return false
		}
	}
	return true
}

type storedPresentation struct {
	cover    pgtype.Text
	featured bool
	rank     int32
	since    pgtype.Timestamptz
}

func (v *storedPresentation) apply(p Presentation) {
	if p.CoverImageURL != nil {
		v.cover = optionalText(*p.CoverImageURL)
	}
	if p.FeaturedRank != nil {
		v.rank = *p.FeaturedRank
	}
	if p.Featured != nil {
		v.featured = *p.Featured
		if v.featured && !v.since.Valid {
			v.since = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
		}
	}
}
