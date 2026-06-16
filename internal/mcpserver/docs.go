package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	resourceConcepts   = "loinc://concepts"
	resourceAgentGuide = "loinc://agent-guide"
	resourceLicense    = "loinc://license-note"
	resourceAPIGuide   = "loinc://api-guide"
)

var conceptDocFiles = []string{
	"LOINC_CONCEPTS.md",
	"LOINC_TERM_STRUCTURE.md",
	"LOINC_NAMES_AND_DISPLAY.md",
	"LOINC_SPECIAL_CASES.md",
	"LOINC_DATABASE_STRUCTURE.md",
	"LOINC_PART_LINKAGES.md",
	"LOINC_OFFICIAL_API.md",
	"LOINC_LICENSE_NOTE.md",
}

type Docs struct {
	dir string
}

type ConceptRequest struct {
	Topic  string `json:"topic,omitempty" jsonschema:"LOINC concept topic, such as major_parts, names, status, usage, panels, answer_lists, special_cases, microbiology, antimicrobial_susceptibility, database_structure, part_linkages, primary_linkages, semantic_enhancement, map_to_table, source_organization, copyright, or search_strategy"`
	Detail string `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
}

type TextResponse struct {
	Text string `json:"text"`
}

type markdownSection struct {
	Title string
	Slug  string
	Body  string
}

func NewDocs(dir string) *Docs {
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join(".", "docs", "agent")
	}
	return &Docs{dir: dir}
}

func (d *Docs) ExplainConcept(ctx context.Context, req ConceptRequest) (TextResponse, error) {
	sections, err := d.conceptSections(ctx)
	if err != nil {
		return TextResponse{}, err
	}
	if strings.TrimSpace(req.Topic) == "" {
		return TextResponse{Text: conceptIndex(sections)}, nil
	}
	want := topicSlug(req.Topic)
	for _, section := range sections {
		if section.Slug == want {
			if req.Detail == "full" {
				return TextResponse{Text: "## " + section.Title + "\n\n" + strings.TrimSpace(section.Body)}, nil
			}
			return TextResponse{Text: strings.TrimSpace(section.Body)}, nil
		}
	}
	return TextResponse{}, fmt.Errorf("LOINC concept topic %q not found in %s", req.Topic, d.path("LOINC_CONCEPTS.md"))
}

func (d *Docs) conceptSections(ctx context.Context) ([]markdownSection, error) {
	var sections []markdownSection
	for i, name := range conceptDocFiles {
		text, err := d.readFile(ctx, name)
		if err != nil {
			if i > 0 && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		sections = append(sections, markdownSections(text)...)
	}
	return sections, nil
}

func (d *Docs) ReadResource(ctx context.Context, uri string) (TextResponse, error) {
	switch uri {
	case resourceConcepts:
		return d.readMarkdownResource(ctx, "LOINC_CONCEPTS.md")
	case resourceAgentGuide:
		return d.readMarkdownResource(ctx, "LOINC_AGENT_GUIDE.md")
	case resourceLicense:
		return d.readMarkdownResource(ctx, "LOINC_LICENSE_NOTE.md")
	case resourceAPIGuide:
		return d.readFileResource(ctx, filepath.Join(filepath.Dir(d.dir), "API.md"))
	default:
		return TextResponse{}, fmt.Errorf("unknown LOINC MCP resource %q", uri)
	}
}

func (d *Docs) readMarkdownResource(ctx context.Context, name string) (TextResponse, error) {
	text, err := d.readFile(ctx, name)
	if err != nil {
		return TextResponse{}, err
	}
	return TextResponse{Text: text}, nil
}

func (d *Docs) readFile(ctx context.Context, name string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	path := d.path(name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read LOINC agent docs file %s: %w", path, err)
	}
	return string(data), nil
}

func (d *Docs) readFileResource(ctx context.Context, path string) (TextResponse, error) {
	select {
	case <-ctx.Done():
		return TextResponse{}, ctx.Err()
	default:
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return TextResponse{}, fmt.Errorf("read LOINC agent docs file %s: %w", path, err)
	}
	return TextResponse{Text: string(data)}, nil
}

func (d *Docs) path(name string) string {
	return filepath.Join(d.dir, name)
}

func markdownSections(markdown string) []markdownSection {
	lines := strings.Split(markdown, "\n")
	var sections []markdownSection
	var current *markdownSection
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if current != nil {
				current.Body = strings.TrimSpace(current.Body)
				sections = append(sections, *current)
			}
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			current = &markdownSection{Title: title, Slug: topicSlug(title)}
			continue
		}
		if current != nil {
			current.Body += line + "\n"
		}
	}
	if current != nil {
		current.Body = strings.TrimSpace(current.Body)
		sections = append(sections, *current)
	}
	return sections
}

func conceptIndex(sections []markdownSection) string {
	if len(sections) == 0 {
		return "No LOINC concept sections found."
	}
	lines := make([]string, 0, len(sections)+1)
	lines = append(lines, "Available LOINC concept topics:")
	sort.Slice(sections, func(i, j int) bool { return sections[i].Slug < sections[j].Slug })
	for _, section := range sections {
		lines = append(lines, "- "+section.Slug+": "+section.Title)
	}
	return strings.Join(lines, "\n")
}

var topicSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func topicSlug(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = strings.ReplaceAll(slug, "&", " and ")
	slug = topicSlugPattern.ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")
	return slug
}
