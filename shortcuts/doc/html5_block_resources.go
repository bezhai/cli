// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	html5BlockTag          = "html5-block"
	html5BlockPathAttr     = "path"
	html5BlockDataRefAttr  = "data-ref"
	html5BlockRefDataAttr  = "ref-data"
	html5BlockResourceRoot = "doc-fetch-resources"
)

var (
	html5BlockStartTagPattern = regexp.MustCompile(`(?is)<html5-block\b[^>]*>`)
	html5BlockSafeNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type html5BlockResourceEntry struct {
	Data string `json:"data"`
}

type html5BlockResourceMap map[string]map[string]html5BlockResourceEntry

type html5BlockAttr struct {
	Name  string
	Value string
}

type html5BlockStartTag struct {
	Attrs       []html5BlockAttr
	SelfClosing bool
}

func buildCreateBodyWithHTML5Resources(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	body := buildCreateBody(runtime)
	content, resources, err := prepareHTML5BlockWriteContent(runtime, runtime.Str("doc-format"), runtime.Str("content"))
	if err != nil {
		return nil, err
	}
	body["content"] = content
	if resources != "" {
		body["resources"] = resources
	}
	return body, nil
}

func buildUpdateBodyWithHTML5Resources(runtime *common.RuntimeContext) (map[string]interface{}, error) {
	body := buildUpdateBody(runtime)
	content, resources, err := prepareHTML5BlockWriteContent(runtime, runtime.Str("doc-format"), runtime.Str("content"))
	if err != nil {
		return nil, err
	}
	if content != "" {
		body["content"] = content
	}
	if resources != "" {
		body["resources"] = resources
	}
	return body, nil
}

func validateHTML5BlockWriteContent(runtime *common.RuntimeContext, format string, content string) error {
	_, _, err := prepareHTML5BlockWriteContent(runtime, format, content)
	return err
}

func prepareHTML5BlockWriteContent(runtime *common.RuntimeContext, format string, content string) (string, string, error) {
	if !strings.Contains(content, "<html5-block") {
		return content, "", nil
	}

	resources := html5BlockResourceMap{html5BlockTag: map[string]html5BlockResourceEntry{}}
	nextRef := 1
	rewrite := func(segment string) (string, error) {
		return rewriteHTML5BlockStartTags(segment, func(raw string) (string, error) {
			tag, err := parseHTML5BlockStartTag(raw)
			if err != nil {
				return "", common.ValidationErrorf("invalid html5-block tag: %v", err).WithParam("html5-block")
			}

			pathValue, hasPath := tag.attr(html5BlockPathAttr)
			if tag.hasAttr(html5BlockDataRefAttr) || tag.hasAttr(html5BlockRefDataAttr) {
				return "", common.ValidationErrorf("html5-block in lark-cli input must use path=\"@relative.html\"; data-ref/ref-data is reserved for API and SDK internals").WithParam("html5-block")
			}
			if !hasPath {
				return "", common.ValidationErrorf("html5-block requires path=\"@relative.html\" in lark-cli input").WithParam("html5-block")
			}

			pathRaw := strings.TrimSpace(pathValue)
			if !strings.HasPrefix(pathRaw, "@") {
				return "", common.ValidationErrorf("html5-block path %q must start with @, for example path=\"@./widget.html\"", pathValue).WithParam("path")
			}
			relPath := strings.TrimSpace(strings.TrimPrefix(pathRaw, "@"))
			if relPath == "" {
				return "", common.ValidationErrorf("html5-block path cannot be empty after @").WithParam("path")
			}
			data, err := cmdutil.ReadInputFile(runtime.FileIO(), relPath)
			if err != nil {
				return "", common.ValidationErrorf("html5-block path %q cannot be read: %v", relPath, err).WithParam("path").WithCause(err)
			}

			ref := fmt.Sprintf("html5_%d", nextRef)
			nextRef++
			resources[html5BlockTag][ref] = html5BlockResourceEntry{Data: string(data)}
			tag.removeAttrs(html5BlockPathAttr, html5BlockDataRefAttr, html5BlockRefDataAttr)
			tag.Attrs = append(tag.Attrs, html5BlockAttr{Name: html5BlockDataRefAttr, Value: ref})
			return tag.render(false), nil
		})
	}

	var (
		out string
		err error
	)
	if strings.TrimSpace(format) == "markdown" {
		out = applyOutsideCodeFences(content, func(segment string) string {
			if err != nil {
				return segment
			}
			outSegment, rewriteErr := rewrite(segment)
			if rewriteErr != nil {
				err = rewriteErr
				return segment
			}
			return outSegment
		})
	} else {
		out, err = rewrite(content)
	}
	if err != nil {
		return "", "", err
	}
	if len(resources[html5BlockTag]) == 0 {
		return out, "", nil
	}

	rawResources, err := marshalHTML5BlockResources(resources)
	if err != nil {
		return "", "", err
	}
	return out, rawResources, nil
}

func materializeHTML5BlockResources(runtime *common.RuntimeContext, format string, docToken string, data map[string]interface{}) error {
	doc, _ := data["document"].(map[string]interface{})
	if doc == nil {
		return nil
	}
	content, _ := doc["content"].(string)
	if !hasProcessableHTML5Block(format, content) {
		return nil
	}
	resourcesRaw, _ := doc["resources"].(string)
	resources, err := parseHTML5BlockResources(resourcesRaw)
	if err != nil {
		return err
	}

	wrote := false
	rewrite := func(segment string) (string, error) {
		return rewriteHTML5BlockStartTags(segment, func(raw string) (string, error) {
			return materializeHTML5BlockResourceTag(resources, docToken, raw, func() { wrote = true })
		})
	}

	var (
		rewritten  string
		rewriteErr error
	)
	if strings.TrimSpace(format) == "markdown" {
		rewritten = applyOutsideCodeFences(content, func(segment string) string {
			if rewriteErr != nil {
				return segment
			}
			outSegment, err := rewrite(segment)
			if err != nil {
				rewriteErr = err
				return segment
			}
			return outSegment
		})
	} else {
		rewritten, rewriteErr = rewrite(content)
	}
	if rewriteErr != nil {
		return rewriteErr
	}
	if wrote {
		doc["content"] = rewritten
		delete(doc, "resources")
	}
	return nil
}

func materializeHTML5BlockResourceTag(resources html5BlockResourceMap, docToken string, raw string, markWrote func()) (string, error) {
	tag, err := parseHTML5BlockStartTag(raw)
	if err != nil {
		return "", common.ValidationErrorf("invalid html5-block tag in fetched content: %v", err).WithParam("html5-block")
	}
	ref, ok := tag.attr(html5BlockDataRefAttr)
	if !ok || strings.TrimSpace(ref) == "" {
		return "", common.ValidationErrorf("fetched html5-block is missing data-ref; cannot materialize HTML resource").WithParam("html5-block")
	}
	entry, err := lookupHTML5BlockResource(resources, ref)
	if err != nil {
		return "", err
	}
	relPath, err := writeHTML5BlockResourceFile(docToken, ref, entry.Data)
	if err != nil {
		return "", err
	}

	tag.removeAttrs(html5BlockDataRefAttr, html5BlockRefDataAttr, html5BlockPathAttr)
	tag.Attrs = append(tag.Attrs, html5BlockAttr{Name: html5BlockPathAttr, Value: "@./" + filepath.ToSlash(relPath)})
	markWrote()
	return tag.render(false), nil
}

func hasProcessableHTML5Block(format string, content string) bool {
	if !strings.Contains(content, "<html5-block") {
		return false
	}
	if strings.TrimSpace(format) != "markdown" {
		return true
	}
	found := false
	_ = applyOutsideCodeFences(content, func(segment string) string {
		if strings.Contains(segment, "<html5-block") {
			found = true
		}
		return segment
	})
	return found
}

func parseHTML5BlockResources(raw string) (html5BlockResourceMap, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, common.ValidationErrorf("document.resources is required for fetched html5-block content").WithParam("resources")
	}
	var resources html5BlockResourceMap
	if err := json.Unmarshal([]byte(raw), &resources); err != nil {
		return nil, common.ValidationErrorf("document.resources is not valid html5-block JSON: %v", err).WithParam("resources").WithCause(err)
	}
	if resources[html5BlockTag] == nil {
		return nil, common.ValidationErrorf("document.resources.%s is required for fetched html5-block content", html5BlockTag).WithParam("resources")
	}
	return resources, nil
}

func lookupHTML5BlockResource(resources html5BlockResourceMap, ref string) (html5BlockResourceEntry, error) {
	ref = strings.TrimSpace(ref)
	group := resources[html5BlockTag]
	entry, ok := group[ref]
	if !ok {
		return html5BlockResourceEntry{}, common.ValidationErrorf("document.resources.%s.%s is missing; cannot materialize html5-block", html5BlockTag, ref).WithParam("resources")
	}
	return entry, nil
}

func writeHTML5BlockResourceFile(docToken string, ref string, html string) (string, error) {
	if !html5BlockSafeNamePattern.MatchString(docToken) {
		return "", common.ValidationErrorf("document_id %q cannot be used as a resource directory name", docToken).WithParam("document_id")
	}
	if !html5BlockSafeNamePattern.MatchString(ref) {
		return "", common.ValidationErrorf("html5-block data-ref %q cannot be used as a file name", ref).WithParam("data-ref")
	}
	relPath := filepath.Join(html5BlockResourceRoot, docToken, ref+".html")
	safePath, err := validate.SafeOutputPath(relPath)
	if err != nil {
		return "", common.ValidationErrorf("cannot write html5-block resource %q: %v", relPath, err).WithParam("resources").WithCause(err)
	}
	if err := vfs.MkdirAll(filepath.Dir(safePath), 0o700); err != nil {
		return "", common.ValidationErrorf("cannot create html5-block resource directory %q: %v", filepath.Dir(relPath), err).WithParam("resources").WithCause(err)
	}
	if err := vfs.WriteFile(safePath, []byte(html), 0o600); err != nil {
		return "", common.ValidationErrorf("cannot write html5-block resource file %q: %v", relPath, err).WithParam("resources").WithCause(err)
	}
	return relPath, nil
}

func rewriteHTML5BlockStartTags(content string, fn func(raw string) (string, error)) (string, error) {
	var rewriteErr error
	out := html5BlockStartTagPattern.ReplaceAllStringFunc(content, func(raw string) string {
		if rewriteErr != nil {
			return raw
		}
		rewritten, err := fn(raw)
		if err != nil {
			rewriteErr = err
			return raw
		}
		return rewritten
	})
	if rewriteErr != nil {
		return "", rewriteErr
	}
	return out, nil
}

func parseHTML5BlockStartTag(raw string) (html5BlockStartTag, error) {
	trimmed := strings.TrimSpace(raw)
	selfClosing := strings.HasSuffix(trimmed, "/>")
	decoder := xml.NewDecoder(strings.NewReader(raw))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return html5BlockStartTag{}, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != html5BlockTag {
			return html5BlockStartTag{}, fmt.Errorf("expected <%s>, got <%s>", html5BlockTag, start.Name.Local)
		}
		attrs := make([]html5BlockAttr, 0, len(start.Attr))
		for _, attr := range start.Attr {
			attrs = append(attrs, html5BlockAttr{Name: attr.Name.Local, Value: attr.Value})
		}
		return html5BlockStartTag{Attrs: attrs, SelfClosing: selfClosing}, nil
	}
	return html5BlockStartTag{}, fmt.Errorf("missing start element")
}

func (t html5BlockStartTag) attr(name string) (string, bool) {
	for _, attr := range t.Attrs {
		if attr.Name == name {
			return attr.Value, true
		}
	}
	return "", false
}

func (t html5BlockStartTag) hasAttr(name string) bool {
	_, ok := t.attr(name)
	return ok
}

func (t *html5BlockStartTag) removeAttrs(names ...string) {
	remove := make(map[string]struct{}, len(names))
	for _, name := range names {
		remove[name] = struct{}{}
	}
	attrs := t.Attrs[:0]
	for _, attr := range t.Attrs {
		if _, ok := remove[attr.Name]; ok {
			continue
		}
		attrs = append(attrs, attr)
	}
	t.Attrs = attrs
}

func (t html5BlockStartTag) render(selfClosing bool) string {
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(html5BlockTag)
	for _, attr := range t.Attrs {
		b.WriteByte(' ')
		b.WriteString(attr.Name)
		b.WriteString(`="`)
		b.WriteString(escapeXMLAttr(attr.Value))
		b.WriteByte('"')
	}
	if selfClosing {
		b.WriteString("/>")
	} else {
		b.WriteByte('>')
	}
	if t.SelfClosing && !selfClosing {
		b.WriteString("</")
		b.WriteString(html5BlockTag)
		b.WriteByte('>')
	}
	return b.String()
}

func escapeXMLAttr(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func marshalHTML5BlockResources(resources html5BlockResourceMap) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resources); err != nil {
		return "", common.ValidationErrorf("failed to encode html5-block resources: %v", err).WithCause(err)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}
