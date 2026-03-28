package main

import "testing"

func TestBuildBundlePlanItemDerivesVersionedRefs(t *testing.T) {
	t.Parallel()

	item := buildBundlePlanItem(catalogBundle{
		Slug:              "sales-pipeline",
		Name:              "Sales Pipeline",
		SourceDir:         "sales-pipeline",
		OCIRef:            "ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v<version>",
		RuntimeOCIRef:     "ghcr.io/movebigrocks/mbr-ext-sales-pipeline-runtime:v<version>",
		ReleaseTagPattern: "sales-pipeline-v<version>",
		Kind:              "product",
		Scope:             "workspace",
		Risk:              "standard",
		RuntimeClass:      "service_backed",
		ReleaseChannel:    "beta",
	}, manifest{
		Version:      "0.1.0",
		StorageClass: "owned_schema",
		Runtime: &runtimeManifest{
			Protocol:     "unix_socket_http",
			OCIReference: "ghcr.io/movebigrocks/mbr-ext-sales-pipeline-runtime:v0.1.0",
		},
		Schema: &schemaManifest{
			Name:          "ext_sales_pipeline",
			TargetVersion: "000001",
		},
	})

	if item.ReleaseTag != "sales-pipeline-v0.1.0" {
		t.Fatalf("unexpected release tag %q", item.ReleaseTag)
	}
	if item.PublishTag != "v0.1.0" {
		t.Fatalf("unexpected publish tag %q", item.PublishTag)
	}
	if item.BundleRef != "ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v0.1.0" {
		t.Fatalf("unexpected bundle ref %q", item.BundleRef)
	}
	if item.RuntimeRef != "ghcr.io/movebigrocks/mbr-ext-sales-pipeline-runtime:v0.1.0" {
		t.Fatalf("unexpected runtime ref %q", item.RuntimeRef)
	}
	if item.ReleaseChannel != "beta" {
		t.Fatalf("unexpected release channel %q", item.ReleaseChannel)
	}
}

func TestBuildPublicationEvidenceUsesPublishTagOverride(t *testing.T) {
	t.Parallel()

	item := bundlePlanItem{
		Slug:         "ats",
		Name:         "Applicant Tracking",
		PublishTag:   "v0.8.23",
		BundleImage:  "ghcr.io/movebigrocks/mbr-ext-ats",
		BundleRef:    "ghcr.io/movebigrocks/mbr-ext-ats:v0.8.23",
		RuntimeImage: "ghcr.io/movebigrocks/mbr-ext-ats-runtime",
		RuntimeRef:   "ghcr.io/movebigrocks/mbr-ext-ats-runtime:v0.8.23",
	}

	evidence, err := buildPublicationEvidence("DemandOps", item, releaseOptions{
		PublishTag:    "sha-123456789abc",
		BundleDigest:  "sha256:bundle",
		RuntimeDigest: "sha256:runtime",
		PublishLatest: true,
	})
	if err != nil {
		t.Fatalf("buildPublicationEvidence returned error: %v", err)
	}

	if evidence.Publication.BundleRef != "ghcr.io/movebigrocks/mbr-ext-ats:sha-123456789abc" {
		t.Fatalf("unexpected bundle ref %q", evidence.Publication.BundleRef)
	}
	if evidence.Publication.RuntimeRef != "ghcr.io/movebigrocks/mbr-ext-ats-runtime:sha-123456789abc" {
		t.Fatalf("unexpected runtime ref %q", evidence.Publication.RuntimeRef)
	}
	if evidence.Publication.PublishTag != "sha-123456789abc" {
		t.Fatalf("unexpected publish tag %q", evidence.Publication.PublishTag)
	}
	if !evidence.Publication.PublishLatest {
		t.Fatalf("expected publishLatest to be true")
	}
}
