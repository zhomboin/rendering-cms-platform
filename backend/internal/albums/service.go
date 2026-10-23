package albums

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 照片上传沿用平台统一的 20MB 上限，类型限定为三种图片格式。
const maxPhotoUploadBytes = 20 * 1024 * 1024

var (
	allowedPhotoTypes = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	albumSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

const (
	maxAlbumSlugLength       = 128
	maxAlbumTitleRunes       = 200
	maxAlbumDescriptionRunes = 2000
	maxAlbumTagCount         = 20
	maxAlbumTagRunes         = 50
	maxPhotoAltRunes         = 500
	maxPhotoCaptionRunes     = 1000
)

var (
	errAlbumSlugRequired        = errors.New("相册 slug 不能为空")
	errAlbumTitleRequired       = errors.New("相册标题不能为空")
	errAlbumDescriptionRequired = errors.New("相册导语不能为空")
	errPhotoAltRequired         = errors.New("照片 alt 不能为空")
)

func ValidAlbumSlug(slug string) bool {
	return slug != "" &&
		utf8.RuneCountInString(slug) <= maxAlbumSlugLength &&
		albumSlugPattern.MatchString(slug)
}

// ValidateAlbumInput 校验相册元数据；title/description 为中文必填字段，
// titleEn/descriptionEn 可选，缺失时公开接口由 Rendering 回退中文字段。
func ValidateAlbumInput(slug string, title string, description string) error {
	if !ValidAlbumSlug(slug) {
		return errAlbumSlugRequired
	}
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxAlbumTitleRunes {
		return errAlbumTitleRequired
	}
	description = strings.TrimSpace(description)
	if description == "" || utf8.RuneCountInString(description) > maxAlbumDescriptionRunes {
		return errAlbumDescriptionRequired
	}
	return nil
}

func ValidateOptionalText(value string, maxRunes int) error {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return errors.New("字段长度超出限制")
	}
	return nil
}

func ValidateTagList(tags []string) error {
	if len(tags) > maxAlbumTagCount {
		return errors.New("相册标签数量不能超过 20 个")
	}
	for _, tag := range tags {
		if tag == "" || utf8.RuneCountInString(tag) > maxAlbumTagRunes {
			return errors.New("相册标签不能为空且不能超过 50 个字符")
		}
	}
	return nil
}

func ValidatePhotoAlt(alt string) error {
	alt = strings.TrimSpace(alt)
	if alt == "" || utf8.RuneCountInString(alt) > maxPhotoAltRunes {
		return errPhotoAltRequired
	}
	return nil
}

func ValidatePhotoCaption(caption string) error {
	if utf8.RuneCountInString(caption) > maxPhotoCaptionRunes {
		return errors.New("照片说明不能超过 1000 个字符")
	}
	return nil
}

func ValidPhotoContentType(contentType string) bool {
	return allowedPhotoTypes[contentType]
}

func ValidPhotoUploadSize(byteSize int) bool {
	return byteSize > 0 && byteSize <= maxPhotoUploadBytes
}

// ExtensionForContentType 返回按魔数判定后的权威扩展名，不信任客户端文件名。
func ExtensionForContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
