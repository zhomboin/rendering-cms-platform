package articlemcp

import (
	"encoding/json"
	"errors"
	"slices"
)

const (
	createTool  = "create_draft"
	readTool    = "get_article"
	editTool    = "update_draft"
	statusTool  = "article_status"
	publishTool = "publish_article"
)

var draftTools = []string{createTool, readTool, editTool, statusTool}

// nil 是省略字段，空列表是明确收回全部工具。
type permissionList []string

func (p *permissionList) UnmarshalJSON(raw []byte) error {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil || names == nil {
		return errors.New("权限必须是列表")
	}
	ordered := allPermissions(draftTools, true)
	result := make(permissionList, 0, len(ordered))
	for _, name := range ordered {
		if slices.Contains(names, name) {
			result = append(result, name)
		}
	}
	if len(result) != len(names) {
		return errors.New("权限无效或重复")
	}
	*p = result
	return nil
}

func allPermissions(drafts []string, publish bool) []string {
	out := make([]string, 0, len(drafts)+1)
	out = append(out, drafts...)
	if publish {
		out = append(out, publishTool)
	}
	return out
}

func (p permissionList) split() ([]string, bool) {
	publish := slices.Contains(p, publishTool)
	drafts := make([]string, 0, len(p))
	for _, name := range p {
		if name != publishTool {
			drafts = append(drafts, name)
		}
	}
	return drafts, publish
}
