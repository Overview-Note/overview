// Package cli exposes the Overview service layer through command-line
// commands. It runs fully offline (no HTTP server): commands open the vault
// directly, so Overview can be driven by scripts and tools as a plain
// Markdown editor/index over a folder of files.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Overview-Note/overview/internal/config"
	"github.com/Overview-Note/overview/internal/core"
	"github.com/Overview-Note/overview/internal/history"
	"github.com/Overview-Note/overview/internal/index"
	"github.com/Overview-Note/overview/internal/service"
	"github.com/Overview-Note/overview/internal/store"
	"github.com/Overview-Note/overview/internal/trash"
)

// Env carries the process streams so Run is testable.
type Env struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run dispatches a CLI subcommand (everything after the program name). It
// returns a process exit code: 0 on success, 1 on error, 2 on usage errors.
func Run(args []string, env Env) int {
	args, globals := extractGlobals(args)
	cfg := config.Load()
	if globals.dataDir != "" {
		cfg = cfg.WithDataDir(globals.dataDir)
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(env.Stdout)
		return 0
	}

	cmd, rest := args[0], args[1:]
	ctx := context.Background()

	svc, closer, err := openService(cfg)
	if err != nil {
		return fail(env, err)
	}
	defer closer()

	switch cmd {
	case "list", "tree", "ls":
		return cmdList(ctx, svc, globals, env)
	case "search", "find":
		return cmdSearch(ctx, svc, globals, rest, env)
	case "read", "cat":
		return cmdRead(ctx, svc, rest, env)
	case "get", "show":
		return cmdGet(ctx, svc, rest, env)
	case "write", "save", "put":
		return cmdWrite(ctx, svc, rest, env)
	case "delete", "rm":
		return cmdDelete(ctx, svc, rest, env)
	case "move", "mv":
		return cmdMove(ctx, svc, rest, env)
	case "rename":
		return cmdRename(ctx, svc, rest, env)
	case "mkdir":
		return cmdMkdir(ctx, svc, rest, env)
	case "links":
		return cmdLinks(ctx, svc, globals, rest, env)
	case "resolve":
		return cmdResolve(ctx, svc, rest, env)
	case "history", "log":
		return cmdHistory(ctx, svc, globals, rest, env)
	case "revision", "rev":
		return cmdRevision(ctx, svc, rest, env)
	case "restore":
		return cmdRestore(ctx, svc, rest, env)
	case "trash", "trash-list":
		return cmdTrashList(ctx, svc, globals, env)
	case "trash-restore":
		return cmdTrashRestore(ctx, svc, rest, env)
	case "trash-purge":
		return cmdTrashPurge(ctx, svc, rest, env)
	case "asset-upload", "upload":
		return cmdUpload(ctx, svc, rest, env)
	case "assets-orphans", "orphans":
		return cmdOrphans(ctx, svc, globals, env)
	case "assets-purge", "purge-assets":
		return cmdPurgeAssets(ctx, svc, env)
	case "import":
		return cmdImport(ctx, svc, rest, env)
	case "export-zip", "pack":
		return cmdExportZip(ctx, svc, rest, env)
	default:
		fmt.Fprintf(env.Stderr, "overview: unknown command %q\n\n", cmd)
		printUsage(env.Stderr)
		return 2
	}
}

func fail(env Env, err error) int {
	fmt.Fprintln(env.Stderr, "overview:", err)
	return 1
}

// ---------------------------------------------------------------------------
// global flags
// ---------------------------------------------------------------------------

type globals struct {
	json    bool
	dataDir string
}

// extractGlobals pulls --json and --data-dir/-C out of the argument list so
// they may appear anywhere (before or after the subcommand).
func extractGlobals(args []string) ([]string, globals) {
	var g globals
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			g.json = true
		case a == "--data-dir" || a == "-C":
			if i+1 < len(args) {
				g.dataDir = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--data-dir="):
			g.dataDir = strings.TrimPrefix(a, "--data-dir=")
		default:
			out = append(out, a)
		}
	}
	return out, g
}

// ---------------------------------------------------------------------------
// service bootstrap
// ---------------------------------------------------------------------------

func openService(cfg config.Config) (*service.Service, func(), error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, func() {}, fmt.Errorf("prepare data dirs: %w", err)
	}
	st := store.New(cfg.NotesDir, cfg.AssetsDir)
	idx, err := index.Open(cfg.DBPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open index: %w", err)
	}
	hist := history.New(cfg.HistoryDir, cfg.HistoryKeep)
	tr := trash.New(cfg.TrashDir)
	svc := service.New(st, idx, st, hist, tr)
	return svc, func() { _ = idx.Close() }, nil
}

// ---------------------------------------------------------------------------
// output helpers
// ---------------------------------------------------------------------------

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func flagSet(name string, env Env) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	return fs
}

func isSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func parseArgs(fs *flag.FlagSet, args []string) error {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			} else if !isBoolFlag(fs, name) && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	return fs.Parse(append(flags, positional...))
}

func isBoolFlag(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
		return bf.IsBoolFlag()
	}
	return false
}

func requireArgs(env Env, fs *flag.FlagSet, args []string, n int, usage string) bool {
	if len(args) < n {
		fmt.Fprintf(env.Stderr, "usage: overview %s\n", usage)
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// commands
// ---------------------------------------------------------------------------

func cmdList(ctx context.Context, svc *service.Service, g globals, env Env) int {
	tree, err := svc.Tree(ctx)
	if err != nil {
		return fail(env, err)
	}
	if g.json {
		if err := writeJSON(env.Stdout, tree); err != nil {
			return fail(env, err)
		}
		return 0
	}
	printTree(env.Stdout, tree, 0)
	return 0
}

func printTree(w io.Writer, nodes []core.TreeNode, depth int) {
	for _, n := range nodes {
		indent := strings.Repeat("  ", depth)
		if n.Type == "folder" {
			fmt.Fprintf(w, "%s%s/\n", indent, n.Name)
			printTree(w, n.Children, depth+1)
			continue
		}
		fmt.Fprintf(w, "%s%s\n", indent, n.Path)
	}
}

func cmdSearch(ctx context.Context, svc *service.Service, g globals, args []string, env Env) int {
	fs := flagSet("search", env)
	limit := fs.Int("limit", 20, "max results")
	offset := fs.Int("offset", 0, "result offset")
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "search <query> [--limit N] [--offset N]") {
		return 2
	}
	query := strings.Join(rest, " ")
	hits, err := svc.Search(ctx, query, *limit, *offset)
	if err != nil {
		return fail(env, err)
	}
	if g.json {
		if err := writeJSON(env.Stdout, hits); err != nil {
			return fail(env, err)
		}
		return 0
	}
	for _, h := range hits {
		fmt.Fprintf(env.Stdout, "%s\t%s\n", h.Path, stripTags(h.Snippet))
	}
	return 0
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func cmdRead(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("read", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "read <path>") {
		return 2
	}
	note, err := svc.GetNote(ctx, rest[0])
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprint(env.Stdout, note.Body)
	return 0
}

func cmdGet(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("get", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "get <path>") {
		return 2
	}
	note, err := svc.GetNote(ctx, rest[0])
	if err != nil {
		return fail(env, err)
	}
	if err := writeJSON(env.Stdout, note); err != nil {
		return fail(env, err)
	}
	return 0
}

func cmdWrite(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("write", env)
	body := fs.String("body", "", "note body (inline)")
	file := fs.String("file", "", "read the note body from a file")
	stdin := fs.Bool("stdin", false, "read the note body from stdin")
	public := fs.Bool("public", false, "mark the note public")
	create := fs.Bool("create", false, "fail if the note already exists")
	ifVersion := fs.String("if-version", "", "expected current version (optimistic concurrency)")
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "write <path> [--file F | --stdin | --body S] [--public] [--create]") {
		return 2
	}
	path := rest[0]

	content, err := readBody(env, fs, *body, *file, *stdin)
	if err != nil {
		return fail(env, err)
	}

	version := *ifVersion
	if version == "" {
		switch {
		case *create:
			version = "*"
		default:
			if cur, err := svc.GetNote(ctx, path); err == nil {
				version = cur.Version
			} else {
				version = "*"
			}
		}
	}

	note, err := svc.SaveNote(ctx, path, content, version, *public)
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "%s\t%s\n", note.Path, note.Version)
	return 0
}

func readBody(env Env, fs *flag.FlagSet, body, file string, stdin bool) (string, error) {
	switch {
	case isSet(fs, "body"):
		return body, nil
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case stdin:
		b, err := io.ReadAll(env.Stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", nil
	}
}

func cmdDelete(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("delete", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "delete <path>") {
		return 2
	}
	if err := svc.Delete(ctx, rest[0]); err != nil {
		return fail(env, err)
	}
	return 0
}

func cmdMove(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("move", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 2, "move <from> <to>") {
		return 2
	}
	if err := svc.Move(ctx, rest[0], rest[1]); err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "%s -> %s\n", rest[0], rest[1])
	return 0
}

func cmdRename(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("rename", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 2, "rename <path> <new-name>") {
		return 2
	}
	from := rest[0]
	dir := ""
	if i := strings.LastIndex(from, "/"); i >= 0 {
		dir = from[:i+1]
	}
	to := dir + rest[1]
	if err := svc.Move(ctx, from, to); err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "%s -> %s\n", from, to)
	return 0
}

func cmdMkdir(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("mkdir", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "mkdir <path>") {
		return 2
	}
	if err := svc.Mkdir(ctx, rest[0]); err != nil {
		return fail(env, err)
	}
	return 0
}

func cmdLinks(ctx context.Context, svc *service.Service, g globals, args []string, env Env) int {
	fs := flagSet("links", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "links <path>") {
		return 2
	}
	res, err := svc.Links(ctx, rest[0])
	if err != nil {
		return fail(env, err)
	}
	if g.json {
		if err := writeJSON(env.Stdout, res); err != nil {
			return fail(env, err)
		}
		return 0
	}
	fmt.Fprintln(env.Stdout, "outgoing:")
	for _, l := range res.Outgoing {
		target := l.Raw
		if l.Target != nil {
			target = l.Target.Path
		}
		fmt.Fprintf(env.Stdout, "  %s\n", target)
	}
	fmt.Fprintln(env.Stdout, "backlinks:")
	for _, l := range res.Backlinks {
		fmt.Fprintf(env.Stdout, "  %s\n", l.Path)
	}
	return 0
}

func cmdResolve(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("resolve", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "resolve <target>") {
		return 2
	}
	meta, err := svc.ResolveLink(ctx, rest[0])
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprintln(env.Stdout, meta.Path)
	return 0
}

func cmdHistory(ctx context.Context, svc *service.Service, g globals, args []string, env Env) int {
	fs := flagSet("history", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "history <path>") {
		return 2
	}
	revs, err := svc.Revisions(ctx, rest[0])
	if err != nil {
		return fail(env, err)
	}
	if g.json {
		if err := writeJSON(env.Stdout, revs); err != nil {
			return fail(env, err)
		}
		return 0
	}
	for _, r := range revs {
		fmt.Fprintf(env.Stdout, "%s\t%s\t%d\n", r.ID, r.SavedAt.Format(time.RFC3339), r.Size)
	}
	return 0
}

func cmdRevision(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("revision", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 2, "revision <path> <id>") {
		return 2
	}
	body, err := svc.RevisionContent(ctx, rest[0], rest[1])
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprint(env.Stdout, body)
	return 0
}

func cmdRestore(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("restore", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 2, "restore <path> <id>") {
		return 2
	}
	note, err := svc.RestoreRevision(ctx, rest[0], rest[1])
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "%s\t%s\n", note.Path, note.Version)
	return 0
}

func cmdTrashList(ctx context.Context, svc *service.Service, g globals, env Env) int {
	entries, err := svc.Trash(ctx)
	if err != nil {
		return fail(env, err)
	}
	if g.json {
		if err := writeJSON(env.Stdout, entries); err != nil {
			return fail(env, err)
		}
		return 0
	}
	for _, e := range entries {
		fmt.Fprintf(env.Stdout, "%s\t%s\t%s\n", e.ID, e.Original, e.DeletedAt.Format(time.RFC3339))
	}
	return 0
}

func cmdTrashRestore(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("trash-restore", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "trash-restore <id>") {
		return 2
	}
	if err := svc.RestoreTrash(ctx, rest[0]); err != nil {
		return fail(env, err)
	}
	return 0
}

func cmdTrashPurge(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("trash-purge", env)
	all := fs.Bool("all", false, "purge every trashed item")
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if *all {
		entries, err := svc.Trash(ctx)
		if err != nil {
			return fail(env, err)
		}
		for _, e := range entries {
			if err := svc.PurgeTrash(ctx, e.ID); err != nil {
				return fail(env, err)
			}
		}
		fmt.Fprintf(env.Stdout, "purged %d item(s)\n", len(entries))
		return 0
	}
	if !requireArgs(env, fs, rest, 1, "trash-purge <id> | --all") {
		return 2
	}
	if err := svc.PurgeTrash(ctx, rest[0]); err != nil {
		return fail(env, err)
	}
	return 0
}

func cmdUpload(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("asset-upload", env)
	name := fs.String("name", "", "override the stored file name")
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "asset-upload <file> [--name NAME]") {
		return 2
	}
	f, err := os.Open(rest[0])
	if err != nil {
		return fail(env, err)
	}
	defer f.Close()
	assetName := *name
	if assetName == "" {
		assetName = filepath.Base(rest[0])
	}
	asset, err := svc.Upload(ctx, assetName, f)
	if err != nil {
		return fail(env, err)
	}
	if err := writeJSON(env.Stdout, asset); err != nil {
		return fail(env, err)
	}
	return 0
}

func cmdOrphans(ctx context.Context, svc *service.Service, g globals, env Env) int {
	orphans, err := svc.OrphanAssets(ctx)
	if err != nil {
		return fail(env, err)
	}
	if g.json {
		if err := writeJSON(env.Stdout, orphans); err != nil {
			return fail(env, err)
		}
		return 0
	}
	for _, o := range orphans {
		fmt.Fprintln(env.Stdout, o.Path)
	}
	return 0
}

func cmdPurgeAssets(ctx context.Context, svc *service.Service, env Env) int {
	n, err := svc.PurgeOrphanAssets(ctx)
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "purged %d asset(s)\n", n)
	return 0
}

func cmdImport(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("import", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "import <archive.zip>") {
		return 2
	}
	f, err := os.Open(rest[0])
	if err != nil {
		return fail(env, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fail(env, err)
	}
	res, err := svc.ImportArchive(ctx, f, info.Size())
	if err != nil {
		return fail(env, err)
	}
	fmt.Fprintf(env.Stdout, "imported %d note(s), %d asset(s)\n", res.Notes, res.Assets)
	return 0
}

func cmdExportZip(ctx context.Context, svc *service.Service, args []string, env Env) int {
	fs := flagSet("export-zip", env)
	if err := parseArgs(fs, args); err != nil {
		return 2
	}
	rest := fs.Args()
	if !requireArgs(env, fs, rest, 1, "export-zip <archive.zip>") {
		return 2
	}
	f, err := os.Create(rest[0])
	if err != nil {
		return fail(env, err)
	}
	defer f.Close()
	if err := svc.ExportArchive(ctx, f); err != nil {
		return fail(env, err)
	}
	fmt.Fprintln(env.Stdout, rest[0])
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Overview - self-hosted Markdown knowledge base

Usage:
  overview [global flags] <command> [args]

Global flags:
  -C, --data-dir DIR   vault data directory (default $OVERVIEW_DATA_DIR or ./data)
      --json           machine-readable output where supported

Server:
  serve                              start the HTTP server (default :5230)
  export                             export public notes to a static site
  version                            print the version
  help                               show this help

Notes:
  list | tree | ls                   list the note tree (--json)
  search <query> [--limit N]         full-text search
  read <path>                        print a note's Markdown body
  get <path>                         print a note's full metadata + body as JSON
  write <path> [--file F|--stdin|--body S] [--public] [--create] [--if-version V]
  delete <path>                      move a note or folder to the trash
  move <from> <to>                   move/rename a path
  rename <path> <new-name>           rename within the same folder
  mkdir <path>                       create a folder
  links <path>                       outgoing links and backlinks
  resolve <target>                   resolve a [[wiki]] target to a path

History and trash:
  history <path>                     list revisions (--json)
  revision <path> <id>               print a revision's body
  restore <path> <id>                restore a revision
  trash                              list trashed items (--json)
  trash-restore <id>                 restore a trashed item
  trash-purge <id> | --all           permanently delete trashed items

Assets and archives:
  asset-upload <file> [--name N]     upload an attachment (JSON result)
  assets-orphans                     list unreferenced assets (--json)
  assets-purge                       delete unreferenced assets
  import <archive.zip>               import a vault ZIP
  export-zip <archive.zip>           export the vault as a ZIP

Configuration is via OVERVIEW_* environment variables (see docs/DESIGN.md).
`)
}
