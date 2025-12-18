package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"sort"
	"strings"
)

//go:embed templates/*
var templatesFS embed.FS

var (
	htmlTpl = template.Must(template.ParseFS(templatesFS, "templates/html.tmpl"))
	mdTpl   = template.Must(template.ParseFS(templatesFS, "templates/md.tmpl"))
)

type ReportData struct {
	Total struct {
		Versions int
		Tests    int
		Success  int
		Failed   int
	}
	Versions []ReportVersion
	CSSRules template.CSS
}

type ReportVersion struct {
	ID      string
	Name    string
	Summary struct {
		Total   int
		Success int
		Failed  int
	}
	Groups []ReportGroup
}

type ReportGroup struct {
	State     string
	Direction string
	Rows      []ReportRow
}

type ReportRow struct {
	State      string
	Direction  string
	PacketName string
	PacketID   int32
	Success    bool
	Notes      string
}

func renderHTMLTemplate(data ReportData) (string, error) {
	var buf bytes.Buffer
	if err := htmlTpl.ExecuteTemplate(&buf, "html.tmpl", data); err != nil {
		return "", fmt.Errorf("render HTML template: %w", err)
	}
	return buf.String(), nil
}

func renderMDTemplate(data ReportData) (string, error) {
	var buf bytes.Buffer
	if err := mdTpl.ExecuteTemplate(&buf, "md.tmpl", data); err != nil {
		return "", fmt.Errorf("render MD template: %w", err)
	}
	return buf.String(), nil
}

// buildReportData constructs the view model with per-state and per-direction groups
func buildReportData(b struct {
	Versions []versionBlock `json:"versions"`
	Total    struct {
		Versions int `json:"versions"`
		Tests    int `json:"tests"`
		Success  int `json:"success"`
		Failed   int `json:"failed"`
	} `json:"total"`
}) ReportData {
	rd := ReportData{}
	rd.Total.Versions = b.Total.Versions
	rd.Total.Tests = b.Total.Tests
	rd.Total.Success = b.Total.Success
	rd.Total.Failed = b.Total.Failed

	for _, vb := range b.Versions {
		rv := ReportVersion{ID: anchorID(vb.Version), Name: vb.Version}
		rv.Summary.Total = vb.Summary.Total
		rv.Summary.Success = vb.Summary.Success
		rv.Summary.Failed = vb.Summary.Failed

		groups := map[string]*ReportGroup{}
		for _, r := range vb.Results {
			key := r.State + "|" + r.Direction
			g, ok := groups[key]
			if !ok {
				g = &ReportGroup{State: r.State, Direction: r.Direction}
				groups[key] = g
			}
			g.Rows = append(g.Rows, ReportRow{
				State:      r.State,
				Direction:  r.Direction,
				PacketName: r.PacketName,
				PacketID:   r.PacketID,
				Success:    r.Success,
				Notes:      strings.Join(r.Errors, "\n"),
			})
		}
		// Sort groups by desired state order, then direction
		var groupList []ReportGroup
		for _, g := range groups {
			groupList = append(groupList, *g)
		}
		stateOrder := map[string]int{
			"handshaking":   0,
			"login":         1,
			"configuration": 2,
			"status":        3,
			"play":          4,
		}
		stateRank := func(s string) int {
			if v, ok := stateOrder[strings.ToLower(s)]; ok {
				return v
			}
			return 99
		}
		sort.Slice(groupList, func(i, j int) bool {
			ri, rj := stateRank(groupList[i].State), stateRank(groupList[j].State)
			if ri != rj {
				return ri < rj
			}
			if groupList[i].Direction != groupList[j].Direction {
				return groupList[i].Direction < groupList[j].Direction
			}
			// Final tie-breaker
			if groupList[i].State != groupList[j].State {
				return groupList[i].State < groupList[j].State
			}
			return false
		})
		rv.Groups = groupList
		rd.Versions = append(rd.Versions, rv)
	}

	var css strings.Builder
	css.WriteString("#sel-all:checked ~ .layout .content .version { display: block; }\n")
	for _, v := range rd.Versions {
		css.WriteString(fmt.Sprintf("#sel-%s:checked ~ .layout .content #%s { display: block; }\n", v.ID, v.ID))
	}
	rd.CSSRules = template.CSS(css.String())

	return rd
}
