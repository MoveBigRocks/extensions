package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type catalog struct {
	Publisher string          `json:"publisher"`
	Bundles   []catalogBundle `json:"bundles"`
}

type catalogBundle struct {
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	SourceDir         string `json:"sourceDir"`
	OCIRef            string `json:"ociRef"`
	RuntimeOCIRef     string `json:"runtimeOciRef"`
	ReleaseTagPattern string `json:"releaseTagPattern"`
	Kind              string `json:"kind"`
	Scope             string `json:"scope"`
	Risk              string `json:"risk"`
	RuntimeClass      string `json:"runtimeClass"`
	ReleaseChannel    string `json:"releaseChannel"`
}

type manifest struct {
	Version      string           `json:"version"`
	RuntimeClass string           `json:"runtimeClass"`
	StorageClass string           `json:"storageClass"`
	Runtime      *runtimeManifest `json:"runtime,omitempty"`
	Schema       *schemaManifest  `json:"schema,omitempty"`
}

type runtimeManifest struct {
	Protocol     string `json:"protocol"`
	OCIReference string `json:"ociReference"`
	Digest       string `json:"digest"`
}

type schemaManifest struct {
	Name          string `json:"name"`
	TargetVersion string `json:"targetVersion"`
}

type publicationPlan struct {
	GeneratedAt string           `json:"generatedAt"`
	Publisher   string           `json:"publisher"`
	Bundles     []bundlePlanItem `json:"bundles"`
}

type bundlePlanItem struct {
	Slug                string `json:"slug"`
	Name                string `json:"name"`
	SourceDir           string `json:"sourceDir"`
	Version             string `json:"version"`
	ReleaseChannel      string `json:"releaseChannel"`
	ReleaseTag          string `json:"releaseTag"`
	PublishTag          string `json:"publishTag"`
	BundleImage         string `json:"bundleImage"`
	BundleRef           string `json:"bundleRef"`
	RuntimeImage        string `json:"runtimeImage,omitempty"`
	RuntimeRef          string `json:"runtimeRef,omitempty"`
	Kind                string `json:"kind"`
	Scope               string `json:"scope"`
	Risk                string `json:"risk"`
	RuntimeClass        string `json:"runtimeClass"`
	RuntimeProtocol     string `json:"runtimeProtocol,omitempty"`
	StorageClass        string `json:"storageClass,omitempty"`
	SchemaName          string `json:"schemaName,omitempty"`
	SchemaTargetVersion string `json:"schemaTargetVersion,omitempty"`
}

type publicationEvidence struct {
	GeneratedAt string             `json:"generatedAt"`
	Publisher   string             `json:"publisher"`
	Extension   bundlePlanItem     `json:"extension"`
	Publication publicationDetails `json:"publication"`
}

type publicationDetails struct {
	Repository     string `json:"repository,omitempty"`
	EventName      string `json:"eventName,omitempty"`
	GitSHA         string `json:"gitSha,omitempty"`
	WorkflowRunURL string `json:"workflowRunUrl,omitempty"`
	PublishedAt    string `json:"publishedAt,omitempty"`
	PublishTag     string `json:"publishTag"`
	PublishLatest  bool   `json:"publishLatest"`
	BundleRef      string `json:"bundleRef"`
	BundleDigest   string `json:"bundleDigest"`
	RuntimeRef     string `json:"runtimeRef,omitempty"`
	RuntimeDigest  string `json:"runtimeDigest,omitempty"`
}

type releaseOptions struct {
	Repository     string
	EventName      string
	GitSHA         string
	WorkflowRunURL string
	PublishedAt    string
	PublishTag     string
	PublishLatest  bool
	BundleRef      string
	BundleDigest   string
	RuntimeRef     string
	RuntimeDigest  string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	mode := flag.String("mode", "plan", "plan or release")
	sourceRoot := flag.String("source-root", ".", "repo root containing catalog/public-bundles.json")
	catalogPath := flag.String("catalog", "", "optional catalog path override")
	slug := flag.String("slug", "", "bundle slug for release mode")
	outPath := flag.String("out", "", "path to write JSON output")
	repository := flag.String("repository", "", "source repository name")
	eventName := flag.String("event-name", "", "workflow event name")
	gitSHA := flag.String("git-sha", "", "git sha for the release")
	workflowRunURL := flag.String("workflow-run-url", "", "workflow run URL")
	publishedAt := flag.String("published-at", "", "publication timestamp (RFC3339)")
	publishTag := flag.String("publish-tag", "", "published OCI tag")
	publishLatest := flag.Bool("publish-latest", false, "whether latest was also tagged")
	bundleRef := flag.String("bundle-ref", "", "published bundle ref")
	bundleDigest := flag.String("bundle-digest", "", "published bundle digest")
	runtimeRef := flag.String("runtime-ref", "", "published runtime ref")
	runtimeDigest := flag.String("runtime-digest", "", "published runtime digest")
	flag.Parse()

	if strings.TrimSpace(*outPath) == "" {
		return fmt.Errorf("--out is required")
	}

	resolvedCatalogPath := strings.TrimSpace(*catalogPath)
	if resolvedCatalogPath == "" {
		resolvedCatalogPath = filepath.Join(strings.TrimSpace(*sourceRoot), "catalog", "public-bundles.json")
	}
	cat, err := loadCatalog(resolvedCatalogPath)
	if err != nil {
		return err
	}

	switch strings.TrimSpace(*mode) {
	case "plan":
		plan, err := buildPublicationPlan(strings.TrimSpace(*sourceRoot), cat)
		if err != nil {
			return err
		}
		return writeJSON(*outPath, plan)
	case "release":
		if strings.TrimSpace(*slug) == "" {
			return fmt.Errorf("--slug is required for release mode")
		}
		bundle, err := findCatalogBundle(cat, *slug)
		if err != nil {
			return err
		}
		m, err := loadManifest(strings.TrimSpace(*sourceRoot), bundle.SourceDir)
		if err != nil {
			return err
		}
		item := buildBundlePlanItem(bundle, m)
		evidence, err := buildPublicationEvidence(cat.Publisher, item, releaseOptions{
			Repository:     strings.TrimSpace(*repository),
			EventName:      strings.TrimSpace(*eventName),
			GitSHA:         strings.TrimSpace(*gitSHA),
			WorkflowRunURL: strings.TrimSpace(*workflowRunURL),
			PublishedAt:    strings.TrimSpace(*publishedAt),
			PublishTag:     strings.TrimSpace(*publishTag),
			PublishLatest:  *publishLatest,
			BundleRef:      strings.TrimSpace(*bundleRef),
			BundleDigest:   strings.TrimSpace(*bundleDigest),
			RuntimeRef:     strings.TrimSpace(*runtimeRef),
			RuntimeDigest:  strings.TrimSpace(*runtimeDigest),
		})
		if err != nil {
			return err
		}
		return writeJSON(*outPath, evidence)
	default:
		return fmt.Errorf("unsupported mode %q", *mode)
	}
}

func loadCatalog(path string) (catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return catalog{}, fmt.Errorf("read catalog: %w", err)
	}
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	return cat, nil
}

func findCatalogBundle(cat catalog, slug string) (catalogBundle, error) {
	for _, bundle := range cat.Bundles {
		if bundle.Slug == slug {
			return bundle, nil
		}
	}
	return catalogBundle{}, fmt.Errorf("bundle %q not found in catalog", slug)
}

func loadManifest(sourceRoot, sourceDir string) (manifest, error) {
	manifestPath := filepath.Join(sourceRoot, sourceDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return manifest{}, fmt.Errorf("read manifest for %s: %w", sourceDir, err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("decode manifest for %s: %w", sourceDir, err)
	}
	return m, nil
}

func buildPublicationPlan(sourceRoot string, cat catalog) (publicationPlan, error) {
	plan := publicationPlan{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Publisher:   cat.Publisher,
		Bundles:     make([]bundlePlanItem, 0, len(cat.Bundles)),
	}
	for _, bundle := range cat.Bundles {
		m, err := loadManifest(sourceRoot, bundle.SourceDir)
		if err != nil {
			return publicationPlan{}, err
		}
		plan.Bundles = append(plan.Bundles, buildBundlePlanItem(bundle, m))
	}
	return plan, nil
}

func buildBundlePlanItem(bundle catalogBundle, m manifest) bundlePlanItem {
	releaseTag := strings.ReplaceAll(bundle.ReleaseTagPattern, "<version>", m.Version)
	publishTag := releaseTag
	prefix := bundle.Slug + "-"
	if strings.HasPrefix(releaseTag, prefix) {
		publishTag = strings.TrimPrefix(releaseTag, prefix)
	}
	bundleRef := strings.ReplaceAll(bundle.OCIRef, "<version>", m.Version)
	runtimeRef := strings.ReplaceAll(bundle.RuntimeOCIRef, "<version>", m.Version)
	if runtimeRef == "" && m.Runtime != nil {
		runtimeRef = m.Runtime.OCIReference
	}
	item := bundlePlanItem{
		Slug:           bundle.Slug,
		Name:           bundle.Name,
		SourceDir:      bundle.SourceDir,
		Version:        m.Version,
		ReleaseChannel: bundle.ReleaseChannel,
		ReleaseTag:     releaseTag,
		PublishTag:     publishTag,
		BundleImage:    imageRef(bundleRef),
		BundleRef:      bundleRef,
		RuntimeImage:   imageRef(runtimeRef),
		RuntimeRef:     runtimeRef,
		Kind:           bundle.Kind,
		Scope:          bundle.Scope,
		Risk:           bundle.Risk,
		RuntimeClass:   bundle.RuntimeClass,
		StorageClass:   m.StorageClass,
	}
	if m.Runtime != nil {
		item.RuntimeProtocol = m.Runtime.Protocol
	}
	if m.Schema != nil {
		item.SchemaName = m.Schema.Name
		item.SchemaTargetVersion = m.Schema.TargetVersion
	}
	return item
}

func buildPublicationEvidence(publisher string, item bundlePlanItem, opts releaseOptions) (publicationEvidence, error) {
	if strings.TrimSpace(opts.BundleDigest) == "" {
		return publicationEvidence{}, fmt.Errorf("bundle digest is required for release evidence")
	}
	if strings.TrimSpace(opts.PublishedAt) == "" {
		opts.PublishedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(opts.PublishTag) == "" {
		opts.PublishTag = item.PublishTag
	}
	if strings.TrimSpace(opts.BundleRef) == "" {
		opts.BundleRef = item.BundleImage + ":" + opts.PublishTag
	}
	if strings.TrimSpace(opts.RuntimeRef) == "" && item.RuntimeImage != "" {
		opts.RuntimeRef = item.RuntimeImage + ":" + opts.PublishTag
	}

	evidenceItem := item
	evidenceItem.PublishTag = opts.PublishTag
	evidenceItem.BundleRef = opts.BundleRef
	evidenceItem.RuntimeRef = opts.RuntimeRef

	return publicationEvidence{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Publisher:   publisher,
		Extension:   evidenceItem,
		Publication: publicationDetails{
			Repository:     opts.Repository,
			EventName:      opts.EventName,
			GitSHA:         opts.GitSHA,
			WorkflowRunURL: opts.WorkflowRunURL,
			PublishedAt:    opts.PublishedAt,
			PublishTag:     opts.PublishTag,
			PublishLatest:  opts.PublishLatest,
			BundleRef:      opts.BundleRef,
			BundleDigest:   opts.BundleDigest,
			RuntimeRef:     opts.RuntimeRef,
			RuntimeDigest:  opts.RuntimeDigest,
		},
	}, nil
}

func imageRef(ref string) string {
	if ref == "" {
		return ""
	}
	idx := strings.LastIndex(ref, ":")
	if idx <= strings.LastIndex(ref, "/") {
		return ref
	}
	return ref[:idx]
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
