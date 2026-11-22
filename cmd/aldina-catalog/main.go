// Aldina — ProfileAsCode for 3D printing.
// Copyright (C) 2026 Thomas Moreau
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command aldina-catalog regenerates internal/catalog/catalog.json from the
// slicers' upstream profile catalogues.
//
// It is a MAINTENANCE tool, run by hand and committed like a golden file: the
// library and the tests never reach the network (CONTRIBUTING.md §Tests), they read
// the embedded snapshot this command produces.
//
//	go run ./cmd/aldina-catalog -out internal/catalog/catalog.json
//
// One tarball per repository rather than ~1100 GitHub API calls, which would
// hit the 60/h anonymous rate limit long before finishing.
package main

import (
	"archive/tar"
	"cmp"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/thomsmoreau/Aldina/internal/catalog"
)

// The two upstream catalogues. Prusa's own repository ships resources/profiles
// with ONE ini per vendor, mirroring what the installed app reads — simpler and
// more faithful than PrusaSlicer-settings, whose live/ tree is versioned per
// vendor and would need a "latest" heuristic.
const (
	orcaRepo  = "SoftFever/OrcaSlicer"
	prusaRepo = "prusa3d/PrusaSlicer"
)

// httpClient bounds every upstream call. The default client has NO timeout, so
// a server that accepts the connection and then stalls hangs the tool forever
// with no output. Generous on purpose: the Orca tarball is tens of megabytes.
var httpClient = &http.Client{Timeout: 2 * time.Minute}

func main() {
	out := flag.String("out", "internal/catalog/catalog.json", "path of the generated catalogue")
	orcaRef := flag.String("orca-ref", "main", "git ref of "+orcaRepo)
	prusaRef := flag.String("prusa-ref", "master", "git ref of "+prusaRepo)
	flag.Parse()

	if err := run(*out, *orcaRef, *prusaRef); err != nil {
		fmt.Fprintln(os.Stderr, "aldina-catalog:", err)
		os.Exit(1)
	}
}

func run(out, orcaRef, prusaRef string) error {
	// Vendor index files come along for the ride: printer_model and
	// printer_variant live in the machine presets themselves, and parseOrca
	// discards anything whose type is not "machine".
	orcaSrc, orcaFiles, err := fetch(orcaRepo, orcaRef, ".json")
	if err != nil {
		return fmt.Errorf("orca: %w", err)
	}
	orcaEntries := parseOrca(orcaFiles)

	prusaSrc, prusaFiles, err := fetch(prusaRepo, prusaRef, ".ini")
	if err != nil {
		return fmt.Errorf("prusa: %w", err)
	}
	prusaEntries := parsePrusa(prusaFiles)

	snap := catalog.Snapshot{
		Source: map[string]catalog.Source{"orca": orcaSrc, "prusa": prusaSrc},
		Orca:   orcaEntries,
		Prusa:  prusaEntries,
	}
	blob, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, append(blob, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %d orca entries, %d prusa entries\n", out, len(orcaEntries), len(prusaEntries))
	return nil
}

// ---------------------------------------------------------------------------
// Fetch
// ---------------------------------------------------------------------------

// fetch resolves ref to a commit, downloads the repository tarball and returns
// the profile files ending in suffix, keyed by their path INSIDE the repo (the
// tarball's leading "<repo>-<ref>/" component is stripped).
func fetch(repo, ref, suffix string) (catalog.Source, map[string][]byte, error) {
	commit, err := resolveCommit(repo, ref)
	if err != nil {
		return catalog.Source{}, nil, err
	}
	url := fmt.Sprintf("https://codeload.github.com/%s/tar.gz/%s", repo, commit)
	resp, err := httpClient.Get(url)
	if err != nil {
		return catalog.Source{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return catalog.Source{}, nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return catalog.Source{}, nil, err
	}
	defer gz.Close()

	files := make(map[string][]byte)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return catalog.Source{}, nil, err
		}
		if hdr.Typeflag != tar.TypeReg ||
			!strings.Contains(hdr.Name, "/resources/profiles/") ||
			!strings.HasSuffix(hdr.Name, suffix) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return catalog.Source{}, nil, err
		}
		// Strip the archive's top-level directory.
		if i := strings.IndexByte(hdr.Name, '/'); i >= 0 {
			files[hdr.Name[i+1:]] = data
		}
	}
	if len(files) == 0 {
		return catalog.Source{}, nil, fmt.Errorf("no profile file matched in %s@%s", repo, ref)
	}
	src := catalog.Source{Repo: repo, Ref: ref, Commit: commit, Fetched: time.Now().UTC().Format("2006-01-02")}
	return src, files, nil
}

func resolveCommit(repo, ref string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", repo, ref)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.sha")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	return strings.TrimSpace(string(body)), nil
}

// ---------------------------------------------------------------------------
// Orca — resources/profiles/<Vendor>/machine/<preset>.json
// ---------------------------------------------------------------------------

type orcaMachine struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	Instantiation string `json:"instantiation"`
	Inherits      string `json:"inherits"`
	PrinterModel  string `json:"printer_model"`
	// printer_variant IS the nozzle diameter, as a string ("0.6").
	PrinterVariant string `json:"printer_variant"`
}

func parseOrca(files map[string][]byte) []catalog.Machine {
	// Index every machine json by "<vendor>/machine/<name>" so inherits can be
	// walked: a variant preset routinely omits printer_model and gets it from
	// its parent.
	machines := make(map[string]orcaMachine, len(files))
	for p, data := range files {
		if path.Base(path.Dir(p)) != "machine" {
			continue
		}
		var m orcaMachine
		if err := json.Unmarshal(data, &m); err != nil {
			continue // vendor index files and malformed entries are not machines
		}
		if m.Type != "machine" {
			continue
		}
		machines[p] = m
	}

	acc := make(byKey)
	for p, m := range machines {
		// Only instantiable presets are selectable by a user; the fdm_*_common
		// bases exist purely to be inherited from.
		if m.Instantiation != "true" {
			continue
		}
		model, variant := m.PrinterModel, m.PrinterVariant
		cur, dir := m, path.Dir(p)
		for hops := 0; (model == "" || variant == "") && cur.Inherits != "" && hops < 10; hops++ {
			parent, ok := machines[path.Join(dir, cur.Inherits+".json")]
			if !ok {
				break
			}
			cur = parent
			if model == "" {
				model = parent.PrinterModel
			}
			if variant == "" {
				variant = parent.PrinterVariant
			}
		}
		acc.add(model, "", variant, m.Name)
	}
	return acc.entries()
}

// ---------------------------------------------------------------------------
// Prusa — resources/profiles/<Vendor>.ini
// ---------------------------------------------------------------------------

func parsePrusa(files map[string][]byte) []catalog.Machine {
	acc := make(byKey)
	for _, data := range files {
		sections := parseINISections(string(data))

		// [printer_model:MINI] holds the human name; [printer:NAME] holds the
		// id in printer_model. The condition compares against the ID, the
		// author writes the human name — the catalogue carries both.
		modelNames := make(map[string]string)
		printers := make(map[string]map[string]string)
		for name, keys := range sections {
			switch {
			case strings.HasPrefix(name, "printer_model:"):
				id := strings.TrimPrefix(name, "printer_model:")
				// "&&" is the wx menu escape for a literal ampersand: the
				// preset really is "Original Prusa MINI & MINI+".
				modelNames[id] = strings.ReplaceAll(keys["name"], "&&", "&")
			case strings.HasPrefix(name, "printer:"):
				printers[strings.TrimPrefix(name, "printer:")] = keys
			}
		}

		for name := range printers {
			// Sections named *like this* are inheritance templates, never
			// selectable presets.
			if strings.HasPrefix(name, "*") {
				continue
			}
			model, variant := resolvePrusa(printers, name, 0)
			human := modelNames[model]
			if human == "" {
				human = model // vendor declared no printer_model section
			}
			acc.add(human, model, variant, name)
		}
	}
	return acc.entries()
}

// resolvePrusa returns the printer_model / printer_variant a preset ends up
// with. PrusaSlicer's `inherits` is a SEMICOLON-SEPARATED LIST, not a single
// parent: Creality builds every printer out of two mixins
// (`inherits = *ENDER3*; *0.3nozzle*` — the model from one, the nozzle from the
// other). Later parents override earlier ones, and the section's own keys win
// over all of them. Reading it as a single name silently drops whole vendors.
func resolvePrusa(printers map[string]map[string]string, name string, depth int) (model, variant string) {
	keys, ok := printers[name]
	if !ok || depth > 10 { // depth also breaks an inherits cycle
		return "", ""
	}
	for parent := range strings.SplitSeq(keys["inherits"], ";") {
		parent = strings.TrimSpace(parent)
		if parent == "" {
			continue
		}
		pm, pv := resolvePrusa(printers, parent, depth+1)
		if pm != "" {
			model = pm
		}
		if pv != "" {
			variant = pv
		}
	}
	if v := keys["printer_model"]; v != "" {
		model = v
	}
	if v := keys["printer_variant"]; v != "" {
		variant = v
	}
	return model, variant
}

// parseINISections splits a PrusaSlicer vendor bundle into "[section]" → keys.
// Values are taken verbatim: this tool only reads identity keys, none of which
// is quoted or escaped, so the full ini.go dialect is not needed here.
func parseINISections(text string) map[string]map[string]string {
	out := make(map[string]map[string]string)
	var cur map[string]string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			cur = make(map[string]string)
			out[line[1:len(line)-1]] = cur
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || cur == nil {
			continue
		}
		cur[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

// ---------------------------------------------------------------------------
// Accumulation
// ---------------------------------------------------------------------------

type key struct {
	model   string
	modelID string
	nozzle  float64
	variant string
}

// nozzleInVariant pulls the diameter out of a printer_variant. The value is
// usually just the number, but both catalogues also decorate it with a hotend
// class, on either side and with either separator: "HF0.4", "0.4HF",
// "volcano 0.6", "0.4HS". The number is always there; the decoration is what
// the canonical cannot express yet.
var nozzleInVariant = regexp.MustCompile(`[0-9]*\.?[0-9]+`)

// byKey groups presets by everything that identifies them. It duplicates
// catalog.Machine field for field minus Presets, and has to: Go cannot use a
// struct containing a slice as a map key.
type byKey map[key][]string

// add records one preset. Only a preset with no model or no variant at all is
// dropped — it cannot be addressed by anything. A DECORATED variant is kept:
// the index is a faithful copy of the catalogue, and it is Lookup's job to
// refuse what the canonical cannot yet name (see the Variant field).
func (a byKey) add(model, modelID, variant, preset string) {
	if model == "" || variant == "" {
		return
	}
	// Numeric, not textual: Prusa writes "0.50" where Orca writes "0.5".
	nozzle, err := strconv.ParseFloat(nozzleInVariant.FindString(variant), 64)
	if err != nil {
		return
	}
	k := key{model: model, modelID: modelID, nozzle: nozzle, variant: variant}
	a[k] = append(a[k], preset)
}

// entries returns a deterministic list: the file is committed, so an unstable
// order would make every regeneration an unreadable diff.
func (a byKey) entries() []catalog.Machine {
	out := make([]catalog.Machine, 0, len(a))
	for k, presets := range a {
		slices.Sort(presets)
		out = append(out, catalog.Machine{
			Model: k.model, ModelID: k.modelID,
			Nozzle: k.nozzle, Variant: k.variant, Presets: presets,
		})
	}
	slices.SortFunc(out, func(a, b catalog.Machine) int {
		return cmp.Or(
			strings.Compare(a.Model, b.Model),
			cmp.Compare(a.Nozzle, b.Nozzle),
			strings.Compare(a.Variant, b.Variant),
		)
	})
	return out
}
