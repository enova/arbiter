package arbiter

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed public/*
var publicFS embed.FS

// Logger is an interface to be used by arbiter to print log messages
type Logger interface {
	Printf(string, ...interface{})
}

type discardLogger struct{}

func (d *discardLogger) Printf(_ string, _ ...interface{}) {}

// NewHandler returns an http.Handler that serves the arbiter front-end over
// the given BackendList. The BackendList must contain at least one name. If
// the Logger is nil, messages will not be logged anywhere.
func NewHandler(backends BackendList, log Logger) (http.Handler, error) {
	if log == nil {
		log = new(discardLogger)
	}

	names := backends.Names()
	if len(names) == 0 {
		return nil, fmt.Errorf("BackendList must contain at least one name")
	}

	tmpl := template.New("arbiter")
	tmpl.Funcs(template.FuncMap{"prettyJSON": prettyJSON})
	tmpl, err := tmpl.ParseFS(templatesFS, "templates/*.tmpl")

	fmt.Println(tmpl.DefinedTemplates())

	if err != nil {
		return nil, fmt.Errorf("could not parse templates: %w", err)
	}

	public, err := fs.Sub(publicFS, "public")
	if err != nil {
		return nil, fmt.Errorf("could not subsystem public assets: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/search", newSearchHandler(backends, tmpl, log))
	mux.Handle("/", http.FileServer(http.FS(public)))

	return mux, nil
}

type searchHandler struct {
	backends     BackendList
	backendNames []string
	tmpl         *template.Template
	log          Logger
	mux          *http.ServeMux
}

func newSearchHandler(backends BackendList, tmpl *template.Template, log Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSearch(w, r, backends, tmpl, log)
	})
}

type searchView struct {
	BackendNames    []string
	Err             error
	Result          searchResult
	SPath           string
	SelectedBackend string
}

type searchResult struct {
	Outputs          map[string]json.RawMessage
	TerraformVersion string
	Subdirs          map[string]string
}

func handleSearch(w http.ResponseWriter, r *http.Request, backends BackendList, tmpl *template.Template, log Logger) {
	backendNames := backends.Names()
	view := searchView{BackendNames: backendNames}

	spath := r.URL.Query().Get("spath")
	if spath == "" {
		spath = "."
	}

	backend := r.URL.Query().Get("backend")
	if backend == "" {
		backend = backendNames[0]
	}

	view.SPath = spath
	view.SelectedBackend = backend

	log.Printf("searching backend %s for: %s", backend, spath)

	stateFS := backends.GetState(backend)
	if stateFS == nil {
		view.Err = fmt.Errorf(`backend "%s" not found`, backend)
		render(w, view, tmpl, log)
		return
	}

	res, err := executeSearch(stateFS, spath, backend)
	if err != nil {
		log.Printf("failed to execute search: %s", err.Error())
	}

	view.Result = res
	view.Err = err

	render(w, view, tmpl, log)
}

func executeSearch(stateFS fs.FS, spath, backend string) (searchResult, error) {
	sr := searchResult{}

	entries, err := fs.ReadDir(stateFS, spath)
	if err != nil {
		return sr, fmt.Errorf("could not read path contents: %w", err)
	}

	sr.Subdirs = subdirs(entries, spath, backend)

	stateFile := findStateFile(entries)
	if stateFile == "" {
		return sr, nil
	}

	err = populateSearchResults(&sr, path.Join(spath, stateFile), stateFS)

	return sr, nil
}

// TFStateFile represents a terraform state file. We only parse the outputs.
type tfStateFile struct {
	Outputs          map[string]tfOutput `json:"outputs"`
	TerraformVersion string              `json:"terraform_version"`
}

// TFOutput is the value of a single terraform output.
type tfOutput struct {
	Value json.RawMessage `json:"value"`
}

func populateSearchResults(sr *searchResult, stateFile string, stateFS fs.FS) error {
	f, err := stateFS.Open(stateFile)
	if err != nil {
		return fmt.Errorf("could not fetch tf state: %w", err)
	}

	var tfdata tfStateFile
	err = json.NewDecoder(f).Decode(&tfdata)
	if err != nil {
		return fmt.Errorf("could not parse tf state: %w", err)
	}

	sr.TerraformVersion = tfdata.TerraformVersion

	data := map[string]json.RawMessage{}
	for k, v := range tfdata.Outputs {
		data[k] = v.Value
	}

	sr.Outputs = data

	return nil
}

func prettyJSON(r json.RawMessage) (string, error) {
	pretty, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("could not prettify JSON: %w", err)
	}

	return string(pretty), nil
}

func render(w http.ResponseWriter, view searchView, tmpl *template.Template, log Logger) {
	err := tmpl.ExecuteTemplate(w, "search.tmpl", view)
	if err != nil {
		log.Printf("render failed: %s", err.Error())
	}
}

func findStateFile(entries []fs.DirEntry) string {
	for _, e := range entries {
		if path.Ext(e.Name()) == ".tfstate" {
			return e.Name()
		}
	}

	return ""
}

func subdirs(entries []fs.DirEntry, spath, backend string) map[string]string {
	keepers := map[string]string{}

	for _, e := range entries {
		if e.IsDir() {
			key := path.Join(spath, e.Name())

			v := url.Values{}
			v.Add("backend", backend)
			v.Add("spath", key)

			keepers[key] = (&url.URL{
				Path:     "/search",
				RawQuery: v.Encode(),
			}).String()
		}
	}

	return keepers
}
