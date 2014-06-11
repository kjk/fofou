package main

import (
	"bytes"
	"net/http"
	"path/filepath"
	"text/template"
)

var (
	tmplMain      = "main.html"
	tmplForum     = "forum.html"
	tmplTopic     = "topic.html"
	tmplPosts     = "posts.html"
	tmplNewPost   = "newpost.html"
	tmplLogs      = "logs.html"
	templateNames = [...]string{tmplMain, tmplForum, tmplTopic, tmplPosts,
		tmplNewPost, tmplLogs, "footer.html", "analytics.html"}
	templatePaths   []string
	templates       *template.Template
	reloadTemplates = true
)

func GetTemplates() *template.Template {
	if reloadTemplates || (nil == templates) {
		if 0 == len(templatePaths) {
			for _, name := range templateNames {
				templatePaths = append(templatePaths, filepath.Join("tmpl", name))
			}
		}
		templates = template.Must(template.ParseFiles(templatePaths...))
	}
	return templates
}

func ExecTemplate(w http.ResponseWriter, templateName string, model interface{}) bool {
	var buf bytes.Buffer
	if err := GetTemplates().ExecuteTemplate(&buf, templateName, model); err != nil {
		logger.Errorf("Failed to execute template %q, error: %s", templateName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	} else {
		// at this point we ignore error
		w.Write(buf.Bytes())
	}
	return true
}
