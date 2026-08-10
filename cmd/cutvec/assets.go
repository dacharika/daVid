package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tahabsn/cutvec/internal/axerror"
	"github.com/tahabsn/cutvec/internal/project"
	"github.com/tahabsn/cutvec/internal/worker"
)

// Assets are the library half of the edit: the sounds and b-roll an agent
// reaches for once it knows what a moment needs. Finding them is the same
// retrieval problem as finding a moment in a transcript — describe what you
// want, get ranked candidates — so it reuses the same local embeddings and
// LanceDB rather than inventing a second mechanism.
//
// A library is usually named rather than annotated, so an asset's searchable
// text is derived from its path: the filename, humanised, plus the folders
// above it. "sfx/impacts/deep-boom-01.wav" describes itself well enough to
// find, and a sidecar .txt file overrides that when it does not.

// assetExtensions are the media kinds worth indexing. Anything else in a
// library directory (readmes, licences, artwork) is noise in a sound search.
var assetExtensions = map[string]bool{
	".wav": true, ".mp3": true, ".aif": true, ".aiff": true, ".flac": true,
	".m4a": true, ".ogg": true, ".opus": true,
	".mp4": true, ".mov": true, ".mkv": true, ".webm": true,
	".png": true, ".jpg": true, ".jpeg": true,
}

type assetsRequest struct {
	Project string `json:"project"`
	Dir     string `json:"dir"`
	Query   string `json:"query"`
	Limit   int    `json:"limit"`

	Worker     string   `json:"worker"`
	WorkerArgs []string `json:"workerArgs"`
}

type assetHit struct {
	Ref   string  `json:"ref"`
	Text  string  `json:"text"`
	Score float64 `json:"score"`
}

type assetsIndexResult struct {
	Table   string `json:"table"`
	Assets  int    `json:"assets"`
	Dim     int    `json:"dim"`
	Model   string `json:"model,omitempty"`
	Dir     string `json:"dir"`
	Planned bool   `json:"planned,omitempty"`
}

type assetsSearchResult struct {
	Query  string     `json:"query"`
	Assets []assetHit `json:"assets"`
	Model  string     `json:"model,omitempty"`
}

func assetsCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return axerror.New("input.empty", "no assets subcommand given").
			WithHint("run `cutvec assets index <dir>` or `cutvec assets search \"<description>\"`")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "index":
		return assetsIndexCmd(rest, stdin, stdout)
	case "search":
		return assetsSearchCmd(rest, stdin, stdout)
	default:
		return axerror.New("cli.unknown_command",
			fmt.Sprintf("unknown assets subcommand %q", sub)).
			WithHint("the subcommands are `index` and `search`")
	}
}

// describeAsset derives searchable text for a file from its own path, so an
// unannotated library is still searchable. A sidecar .txt wins when present:
// whoever wrote it knows more than the filename does.
func describeAsset(root, path string) string {
	if sidecar, err := readSidecar(path); err == nil && sidecar != "" {
		return sidecar
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	// Folders carry real meaning in a sound library ("sfx/impacts/..."), so they
	// stay part of the description rather than being discarded.
	words := strings.FieldsFunc(rel, func(r rune) bool {
		return r == '/' || r == '_' || r == '-' || r == '.' || r == ' '
	})
	kept := words[:0]
	for _, w := range words {
		// Bare numbers are take numbers, not description.
		if strings.TrimFunc(w, func(r rune) bool { return r >= '0' && r <= '9' }) == "" {
			continue
		}
		kept = append(kept, strings.ToLower(w))
	}
	if len(kept) == 0 {
		return strings.ToLower(filepath.Base(path))
	}
	return strings.Join(kept, " ")
}

func assetsIndexCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posDir, plan, err := parseOnePositional(args, "assets index")
	if err != nil {
		return err
	}
	var req assetsRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posDir != "" {
		req.Dir = posDir
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`set "project" to the project directory the index is stored in`)
	}
	if req.Dir == "" {
		return axerror.New("input.invalid", "asset directory is required").
			WithHint("pass the library directory: `cutvec assets index <dir>`")
	}
	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}

	var items []map[string]any
	walkErr := filepath.WalkDir(req.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // an unreadable entry skips rather than failing the library
		}
		if !assetExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		abs, aerr := filepath.Abs(path)
		if aerr != nil {
			abs = path
		}
		items = append(items, map[string]any{"ref": abs, "text": describeAsset(req.Dir, path)})
		return nil
	})
	if walkErr != nil {
		return axerror.New("assets.unreadable",
			"cannot read the asset directory: "+walkErr.Error()).
			WithHint("check the path exists and is readable").AsRetryable()
	}
	if len(items) == 0 {
		return axerror.New("assets.empty",
			fmt.Sprintf("no indexable media found under %s", req.Dir)).
			WithHint("the library needs audio, video, or image files; other file types are skipped")
	}
	// Sorted so the same library always indexes in the same order, which keeps
	// the table byte-stable between runs.
	sort.Slice(items, func(i, j int) bool {
		return items[i]["ref"].(string) < items[j]["ref"].(string)
	})

	table := filepath.Join(proj.CacheDir(), "assets.lance")
	if plan {
		return emitJSON(stdout, assetsIndexResult{
			Table: table, Assets: len(items), Dir: req.Dir, Planned: true,
		})
	}

	w, err := resolveIndexWorker(indexRequest{Worker: req.Worker, WorkerArgs: req.WorkerArgs})
	if err != nil {
		return err
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op:     "index_assets",
		Params: map[string]any{"items": items, "table": table},
	})
	if err != nil {
		return err
	}
	out := assetsIndexResult{Table: table, Assets: len(items), Dir: req.Dir}
	if n, ok := resp.Result["dim"].(float64); ok {
		out.Dim = int(n)
	}
	if m, ok := resp.Result["model"].(string); ok {
		out.Model = m
	}
	return emitJSON(stdout, out)
}

func assetsSearchCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	posQuery, _, err := parseOnePositional(args, "assets search")
	if err != nil {
		return err
	}
	var req assetsRequest
	if err := decodeOptionalRequest(stdin, &req); err != nil {
		return err
	}
	if posQuery != "" {
		req.Query = posQuery
	}
	if req.Project == "" {
		return axerror.New("input.invalid", "project directory is required").
			WithHint(`set "project" to the project holding the asset index`)
	}
	if req.Query == "" {
		return axerror.New("input.empty", "no query to search assets for").
			WithHint(`describe the sound you want: cutvec assets search "deep impact hit"`)
	}
	proj := project.Open(req.Project)
	if !proj.Initialized() {
		return axerror.New("project.uninitialized", "project is not initialised").
			WithHint("run `cutvec init` on this directory first")
	}

	w, err := resolveSearchWorker(searchRequest{Worker: req.Worker, WorkerArgs: req.WorkerArgs})
	if err != nil {
		return err
	}
	resp, err := w.Invoke(context.Background(), worker.Request{
		Op: "search_assets",
		Params: map[string]any{
			"query": req.Query,
			"table": filepath.Join(proj.CacheDir(), "assets.lance"),
			"limit": req.Limit,
		},
	})
	if err != nil {
		return err
	}
	out := assetsSearchResult{Query: req.Query}
	if m, ok := resp.Result["model"].(string); ok {
		out.Model = m
	}
	if list, ok := resp.Result["assets"].([]any); ok {
		for _, raw := range list {
			row, _ := raw.(map[string]any)
			hit := assetHit{}
			if v, ok := row["ref"].(string); ok {
				hit.Ref = v
			}
			if v, ok := row["text"].(string); ok {
				hit.Text = v
			}
			if v, ok := row["score"].(float64); ok {
				hit.Score = v
			}
			out.Assets = append(out.Assets, hit)
		}
	}
	// Re-rank into a deterministic total order regardless of the worker's
	// iteration order, then cap to the requested limit — mirroring searchCmd's
	// project.RankCandidates so the library retrieval surface is as
	// deterministic as transcript search.
	sort.SliceStable(out.Assets, func(i, j int) bool {
		a, b := out.Assets[i], out.Assets[j]
		switch {
		case a.Score != b.Score:
			return a.Score > b.Score
		case a.Ref != b.Ref:
			return a.Ref < b.Ref
		default:
			return a.Text < b.Text
		}
	})
	if req.Limit > 0 && len(out.Assets) > req.Limit {
		out.Assets = out.Assets[:req.Limit]
	}
	return emitJSON(stdout, out)
}

// readSidecar returns a description written alongside an asset ("boom.wav" ->
// "boom.txt"), which is how a library owner overrides a filename that does not
// describe its contents.
func readSidecar(path string) (string, error) {
	side := strings.TrimSuffix(path, filepath.Ext(path)) + ".txt"
	b, err := os.ReadFile(side)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// parseOnePositional accepts at most one positional argument plus --plan, the
// shape both assets subcommands take.
func parseOnePositional(args []string, verb string) (pos string, plan bool, err error) {
	for _, a := range args {
		switch {
		case a == "--plan":
			plan = true
		case strings.HasPrefix(a, "--"):
			return "", false, axerror.New("cli.unknown_flag",
				fmt.Sprintf("unknown flag %q", a)).
				WithHint("the only flag is --plan (dry-run)")
		case pos != "":
			return "", false, axerror.New("input.invalid",
				verb+" takes at most one argument").
				WithHint("quote a multi-word argument")
		default:
			pos = a
		}
	}
	return pos, plan, nil
}
