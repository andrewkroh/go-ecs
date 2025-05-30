// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// The generator command generates Go code from the ECS flat YAML file.
// It produces a Go file for each ECS version and an index file that
// maps ECS version strings to the field definitions for that version.
package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/dave/jennifer/jen"
	"github.com/elastic/go-licenser/licensing"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mitchellh/hashstructure"
	"gopkg.in/yaml.v3"
)

const (
	appName          = "ecs-generator"
	remoteRepository = "https://github.com/elastic/ecs"
	flatFilePath     = "generated/ecs/ecs_flat.yml"

	license  = "ASL2"
	licensor = "Elasticsearch B.V."
)

var licenseHeader string

func init() {
	var sb strings.Builder
	for _, line := range licensing.Headers[license] {
		if strings.Contains(line, "%s") {
			fmt.Fprintf(&sb, line, licensor)
			sb.WriteByte('\n')
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	licenseHeader = sb.String()
}

// Parameters
var (
	fetch     bool
	dumpYAML  bool
	outputDir string
)

func init() {
	flag.BoolVar(&fetch, "fetch", false, "git fetch latest changes")
	flag.BoolVar(&dumpYAML, "dump", false, "Dump ecs_flat.yml files to versions/ directory.")
	flag.StringVar(&outputDir, "out-dir", "../version", "Directory to output generated Go file to.")
}

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	repo, err := cloneRepo()
	if err != nil {
		return err
	}

	releaseRefs, err := getReleasesTags(repo)
	if err != nil {
		return err
	}

	var allFields []ecsField                    // Every unique ECS field definition.
	versionMarkers := map[int]*semver.Version{} // Index into allFields marking where each ECS version begins.
	fieldSet := map[ecsField]int{}              // Map of ecsField to index within allFields.

	var versions []versionFieldSet
	for _, ref := range releaseRefs {
		if err = checkout(repo, ref); err != nil {
			return err
		}

		wt, err := repo.Worktree()
		if err != nil {
			return err
		}

		ver := tagToSemver(ref)

		if dumpYAML {
			if err = dumpECSFlatToDisk(ver, wt); err != nil {
				return err
			}
		}

		fields, hash, err := readECSFlat(ver, wt)
		if err != nil {
			return err
		}

		versionMarkers[len(allFields)] = ver
		var indices []int // Index into allFields for each field in this version.
		for _, f := range fields {
			idx, found := fieldSet[f]
			if !found {
				// First time we see this field, add it to the list.
				idx = len(allFields)
				allFields = append(allFields, f)
				fieldSet[f] = idx
				indices = append(indices, idx)
			} else {
				indices = append(indices, idx)
			}
		}

		if len(indices) > 0 {
			versions = append(versions, versionFieldSet{
				Version: ver,
				Fields:  indices,
				Hash:    hash,
			})
		}
	}

	if err = deleteExistingOutputFiles(); err != nil {
		return err
	}

	if err = writeAllFieldsArrayGoFile(allFields, versionMarkers); err != nil {
		return err
	}

	// If two versions have the same fields, then don't write those fields and
	// use a version alias.
	identifyDuplicateFieldSets(versions)

	for _, fs := range versions {
		// identifyDuplicateFieldSets sets Fields=nil when it finds a duplicate.
		if fs.Fields != nil {
			if err = writeFieldsVersionGoFile(fs.Version, fs.Fields); err != nil {
				return fmt.Errorf("failed to write version file for %v: %w", fs.Version, err)
			}
		}
	}

	return writeECSVersionIndexGoFile(versions)
}

func cloneRepo() (*git.Repository, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	url, err := url.Parse(remoteRepository)
	if err != nil {
		return nil, err
	}

	appDir := filepath.Join(home, "."+appName)
	repoName := path.Base(url.Path)
	repoDir := filepath.Join(appDir, "git", repoName)

	// Open or clone.
	repo, err := git.PlainOpen(repoDir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		log.Printf("Cloning %v into %v.", repoName, repoDir)
		repo, err = git.PlainClone(repoDir, false, &git.CloneOptions{
			URL: remoteRepository,
		})
	}
	if err != nil {
		return nil, err
	}

	if fetch {
		log.Println("Fetching latest changes.")
		err = repo.Fetch(&git.FetchOptions{})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil, fmt.Errorf("failed in git fetch: %w", err)
		}
		log.Println("Fetch completed.")
	}

	return repo, nil
}

func getReleasesTags(repo *git.Repository) ([]*plumbing.Reference, error) {
	tagItr, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	versionToRef := map[*semver.Version]*plumbing.Reference{}
	err = tagItr.ForEach(func(reference *plumbing.Reference) error {
		ver := tagToSemver(reference)
		if ver == nil || ver.PreRelease != "" {
			return nil
		}

		versionToRef[ver] = reference
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort
	versions := slices.Collect(maps.Keys(versionToRef))
	semver.Sort(versions)

	out := make([]*plumbing.Reference, 0, len(versions))
	for _, ver := range versions {
		out = append(out, versionToRef[ver])
	}

	return out, nil
}

func checkout(repo *git.Repository, ref *plumbing.Reference) error {
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	log.Println("Cleaning repo.")
	err = wt.Clean(&git.CleanOptions{
		Dir: true,
	})
	if err != nil {
		return fmt.Errorf("clean failed: %w", err)
	}

	log.Printf("Checking out %v.", ref)
	err = wt.Checkout(&git.CheckoutOptions{
		Branch: ref.Name(),
		Force:  true,
	})
	if err != nil {
		return fmt.Errorf("checkout failed for %s: %w", ref, err)
	}
	log.Println("Checkout completed.")

	return nil
}

func tagToSemver(ref *plumbing.Reference) *semver.Version {
	tag := ref.Name().Short()

	if !strings.HasPrefix(tag, "v") {
		return nil
	}
	tag = strings.TrimPrefix(tag, "v")

	ver, err := semver.NewVersion(tag)
	if err != nil {
		return nil
	}

	return ver
}

func dumpECSFlatToDisk(ver *semver.Version, wt *git.Worktree) error {
	sourceFile, err := wt.Filesystem.Open(flatFilePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer sourceFile.Close()

	_ = os.MkdirAll("version", 0o700)

	targetFile, err := os.Create(filepath.Join("version", ver.String()+".yml"))
	if err != nil {
		return err
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return err
	}

	return targetFile.Close()
}

func readECSFlat(ver *semver.Version, wt *git.Worktree) ([]ecsField, uint64, error) {
	f, err := wt.Filesystem.Open(flatFilePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("Ignoring v%s because %s does not exist.", ver, flatFilePath)
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	// Decode YAML in strict mode to ensure no important data is missed.
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var fields map[string]field
	if err = dec.Decode(&fields); err != nil {
		return nil, 0, fmt.Errorf("failed to decode %s from v%s: %w", flatFilePath, ver, err)
	}

	// Convert to our simplified field representation.
	ecsFields := make([]ecsField, 0, len(fields))
	for _, f := range fields {
		ecsFields = append(ecsFields, newECSField(f))
	}

	slices.SortFunc(ecsFields, func(a, b ecsField) int {
		return cmp.Compare(a.Name, b.Name)
	})

	h, err := hashstructure.Hash(ecsFields, nil)
	if err != nil {
		return nil, 0, err
	}
	return ecsFields, h, nil
}

func writeAllFieldsArrayGoFile(fields []ecsField, markers map[int]*semver.Version) error {
	goFile := filepath.Join(outputDir, "fields.go")

	out, err := os.Create(goFile)
	if err != nil {
		return err
	}
	defer out.Close()

	return generateAllFieldsArray(fields, markers, out)
}

func writeFieldsVersionGoFile(ver *semver.Version, fields []int) error {
	goFile := filepath.Join(outputDir, "v"+strings.ReplaceAll(ver.String(), ".", "_")+".go")

	out, err := os.Create(goFile)
	if err != nil {
		return err
	}
	defer out.Close()

	return generateVersionFieldMap(ver.String(), fields, out)
}

type versionFieldSet struct {
	Version *semver.Version
	Fields  []int
	Hash    uint64
	SameAs  *semver.Version
}

func writeECSVersionIndexGoFile(fieldSets []versionFieldSet) error {
	goFile := filepath.Join(outputDir, "index.go")
	out, err := os.Create(goFile)
	if err != nil {
		return err
	}
	defer out.Close()

	aliases := buildVersionAliases(fieldSets)
	return generateVersionIndex(aliases, out)
}

// generateAllFieldsArray generates a Go file that contains a list of all ECS fields.
// fields is a list of all unique ECS fields. markers is map of indices into fields
// where a new ECS version begins. The markers are used to add comments to the generated
// code indicating which ECS version a group of fields belongs to.
func generateAllFieldsArray(fields []ecsField, markers map[int]*semver.Version, w io.Writer) error {
	o := jen.Options{
		Open:  "{",
		Close: "}",
		Multi: true,
	}

	f := jen.NewFile("version")

	f.HeaderComment(licenseHeader)

	f.HeaderComment("Code generated by generator, DO NOT EDIT.")

	f.Comment("// f contains a list of fields from all ECS versions.")
	f.Comment("// Fields are defined multiple times when the definition changes between versions.")
	f.Var().Id("f").Op("=").Index(jen.Id("...")).Id("Field").
		CustomFunc(o, func(g *jen.Group) {
			for i, field := range fields {
				if comment, found := markers[i]; found {
					g.Commentf("// ECS v%s", comment)
				}
				fieldsDef(field, g).Op(",")
			}
		})

	return f.Render(w)
}

var mapEntriesOptions = jen.Options{
	Open:      "{",
	Close:     "}",
	Separator: ",",
	Multi:     true,
}

func generateVersionFieldMap(version string, fields []int, w io.Writer) error {
	f := jen.NewFile("version")

	f.HeaderComment(licenseHeader)

	f.HeaderComment("Code generated by generator, DO NOT EDIT.")

	f.Var().Id("v"+strings.ReplaceAll(version, ".", "_")).Op("=").Map(jen.String()).Id("*Field").
		CustomFunc(mapEntriesOptions, func(g *jen.Group) {
			for _, idx := range fields {
				g.Id("f").Index(jen.Lit(idx)).Op(".").Id("Name").Op(":").Id("&f").Index(jen.Lit(idx))
			}
		})

	return f.Render(w)
}

func generateVersionIndex(aliases []versionAlias, w io.Writer) error {
	f := jen.NewFile("version")

	f.HeaderComment(licenseHeader)

	f.HeaderComment("Code generated by generator, DO NOT EDIT.")

	latest := aliases[len(aliases)-1].Version
	f.Var().DefsFunc(func(g *jen.Group) {
		// Latest = v1_2_3
		g.Id("Latest").Op("=").Id(versionIdentifier(latest))

		// Index = map[string]map[string]*Field{ ... }
		g.Id("Index").Op("=").Map(jen.String()).Map(jen.String()).Id("*Field").
			CustomFunc(mapEntriesOptions, func(g *jen.Group) {
				for _, a := range aliases {
					g.Lit(a.Alias).Op(":").Id(versionIdentifier(a.Version))
				}
			})
	})

	return f.Render(w)
}

func versionIdentifier(v *semver.Version) string {
	return "v" + strings.ReplaceAll(v.String(), ".", "_")
}

func fieldsDef(f ecsField, s *jen.Group) *jen.Statement {
	values := []jen.Code{
		jen.Id("Name").Op(":").Lit(f.Name),
		jen.Id("DataType").Op(":").Lit(f.DataType),
	}

	if f.Array {
		values = append(values,
			jen.Id("Array").Op(":").Id(strconv.FormatBool(f.Array)),
		)
	}
	if f.ValidationPattern != "" {
		values = append(values,
			jen.Id("Pattern").Op(":").Lit(f.ValidationPattern),
		)
	}

	values = append(values,
		jen.Id("Description").Op(":").Lit(f.Description),
	)

	return s.Values(values...)
}

func identifyDuplicateFieldSets(fieldSets []versionFieldSet) {
	// Sort latest to oldest.
	slices.SortFunc(fieldSets, func(a, b versionFieldSet) int {
		return a.Version.Compare(*b.Version)
	})
	slices.Reverse(fieldSets)

	hashSet := map[uint64]*semver.Version{}
	for i, fs := range fieldSets {
		if sameAs, found := hashSet[fs.Hash]; found {
			fieldSets[i].Fields = nil
			fieldSets[i].SameAs = sameAs
			continue
		}
		hashSet[fs.Hash] = fs.Version
	}
}

type versionAlias struct {
	Alias   string
	Version *semver.Version
}

// buildVersionAliases returns a map of versions that will point to a specific tag.
// Less complete versions always point to the newest version. This assumes
// that no pre-release versions are included. For example,
//
//	8 -> 8.10.3
//	8.10 -> 8.10.3
//	8.10.3 -> 8.10.3
//	8.10.2 -> 8.10.2
func buildVersionAliases(fieldSets []versionFieldSet) []versionAlias {
	index := map[string]*semver.Version{}
	putIfAbsent := func(key string, ver *semver.Version) {
		if _, found := index[key]; !found {
			index[key] = ver
		}
	}
	for _, fs := range fieldSets {
		target := fs.Version
		if fs.SameAs != nil {
			target = fs.SameAs
		}

		putIfAbsent(fmt.Sprintf("%d.%d.%d", fs.Version.Major, fs.Version.Minor, fs.Version.Patch), target)
		putIfAbsent(fmt.Sprintf("%d.%d", fs.Version.Major, fs.Version.Minor), target)
		putIfAbsent(fmt.Sprintf("%d", fs.Version.Major), target)
	}

	aliases := make([]versionAlias, 0, len(index))
	for k, v := range index {
		aliases = append(aliases, versionAlias{Alias: k, Version: v})
	}
	slices.SortFunc(aliases, func(a, b versionAlias) int {
		if c := a.Version.Compare(*b.Version); c != 0 {
			return c
		}
		return cmp.Compare(a.Alias, b.Alias)
	})
	return aliases
}

func deleteExistingOutputFiles() error {
	matches, err := filepath.Glob(filepath.Join(outputDir, "v*.go"))
	if err != nil {
		return err
	}

	for _, path := range matches {
		if err = os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}
