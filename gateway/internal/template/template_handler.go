package template

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
)

// TemplateHandler holds parsed templates e configurações de rota
type TemplateHandler struct {
	templates *template.Template
	baseRoute string
	aliases   map[string]string
}

// NewTemplateHandler parses todos os arquivos HTML em dirPath
func NewTemplateHandler(dirPath, route string, aliases map[string]string) (*TemplateHandler, error) {
	patterns := filepath.Join(dirPath, "*.html")
	tmpl, err := template.ParseGlob(patterns)
	if err != nil {
		return nil, err
	}
	return &TemplateHandler{
		templates: tmpl,
		baseRoute: route,
		aliases:   aliases,
	}, nil
}

// ServeHTTP escolhe o template a renderizar com base em URL.Path
func (th *TemplateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmplName := "index.html"
	path := strings.TrimPrefix(r.URL.Path, th.baseRoute)
	path = strings.Trim(path, "/")

	if path != "" {
		if mapped, ok := th.aliases[path]; ok {
			tmplName = mapped
		} else if strings.HasSuffix(path, ".html") {
			tmplName = path
		} else {
			tmplName = path + ".html"
		}
	}

	if th.templates.Lookup(tmplName) == nil {
		http.NotFound(w, r)
		return
	}
	if err := th.templates.ExecuteTemplate(w, tmplName, nil); err != nil {
		http.Error(w, "Template rendering error", http.StatusInternalServerError)
	}
}
