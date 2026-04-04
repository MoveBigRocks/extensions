package atsruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	atsdomain "github.com/movebigrocks/extensions/ats/runtime/domain"
)

type careersHomePageData struct {
	Site                 CareersSiteProfile
	OpenJobs             []Vacancy
	GeneralApplyJob      *Vacancy
	Team                 []CareersTeamMember
	Gallery              []CareersGalleryItem
	ResumeUploadsEnabled bool
	HasCustomCSS         bool
	PageURL              string
	OrganizationJSON     template.JS
}

type careersJobPageData struct {
	Site                  CareersSiteProfile
	Job                   Vacancy
	Team                  []CareersTeamMember
	Gallery               []CareersGalleryItem
	OpenJobs              []Vacancy
	AcceptingApplications bool
	ResumeUploadsEnabled  bool
	HasCustomCSS          bool
	PageURL               string
	JobPostingJSON        template.JS
}

type careersUnavailableJobPageData struct {
	Site         CareersSiteProfile
	Job          Vacancy
	OpenJobs     []Vacancy
	HasCustomCSS bool
	PageURL      string
}

func renderCareersSite(bundle *CareersSiteBundle) (map[string][]byte, error) {
	if bundle == nil {
		return nil, fmt.Errorf("careers site bundle is required")
	}
	openJobs := filterOpenJobs(bundle.Jobs)
	generalApplyJob := generalApplicationJob(bundle.Jobs)
	files := map[string][]byte{}

	homeHTML, err := renderTemplate("careers_home", careersHomeTemplate, careersHomePageData{
		Site:                 bundle.Site,
		OpenJobs:             openJobs,
		GeneralApplyJob:      generalApplyJob,
		Team:                 bundle.Team,
		Gallery:              filterGallery(bundle.Gallery, "homepage"),
		ResumeUploadsEnabled: bundle.ResumeUploadsEnabled,
		HasCustomCSS:         bundle.Site.CustomCSSEnabled && strings.TrimSpace(bundle.Site.CustomCSS) != "",
		PageURL:              absoluteURL(bundle.Site.WebsiteURL, "/careers"),
		OrganizationJSON:     marshalJSONLD(buildOrganizationJSONLD(bundle.Site)),
	})
	if err != nil {
		return nil, err
	}
	files["site/index.html"] = homeHTML

	applyAlias, err := renderTemplate("careers_apply_alias", careersApplyAliasTemplate, map[string]any{
		"SiteTitle": bundle.Site.SiteTitle,
	})
	if err != nil {
		return nil, err
	}
	files["site/apply"] = applyAlias

	css, err := renderTemplate("careers_css", careersStylesTemplate, map[string]any{
		"PrimaryColor":    nonBlank(bundle.Site.PrimaryColor, "#0f766e"),
		"AccentColor":     nonBlank(bundle.Site.AccentColor, "#f59e0b"),
		"SurfaceColor":    nonBlank(bundle.Site.SurfaceColor, "#f6f5f0"),
		"BackgroundColor": nonBlank(bundle.Site.BackgroundColor, "#fbfaf6"),
		"TextColor":       nonBlank(bundle.Site.TextColor, "#12211b"),
		"MutedColor":      nonBlank(bundle.Site.MutedColor, "#5f6b65"),
	})
	if err != nil {
		return nil, err
	}
	files["site/assets/site.css"] = css
	files["site/assets/site.js"] = []byte(careersSiteJS)
	if bundle.Site.CustomCSSEnabled && strings.TrimSpace(bundle.Site.CustomCSS) != "" {
		files["site/assets/custom.css"] = []byte(bundle.Site.CustomCSS)
	}

	for _, job := range bundle.Jobs {
		if !job.IsListedPublicly() {
			continue
		}
		var (
			jobHTML []byte
			err     error
		)
		if job.Status == atsdomain.VacancyStatusOpen {
			jobHTML, err = renderTemplate("careers_job", careersJobTemplate, careersJobPageData{
				Site:                  bundle.Site,
				Job:                   job,
				Team:                  bundle.Team,
				Gallery:               filterGallery(bundle.Gallery, "jobs"),
				OpenJobs:              openJobs,
				AcceptingApplications: true,
				ResumeUploadsEnabled:  bundle.ResumeUploadsEnabled,
				HasCustomCSS:          bundle.Site.CustomCSSEnabled && strings.TrimSpace(bundle.Site.CustomCSS) != "",
				PageURL:               absoluteURL(bundle.Site.WebsiteURL, "/careers/jobs/"+job.Slug),
				JobPostingJSON:        marshalJSONLD(buildJobPostingJSONLD(bundle.Site, job)),
			})
		} else if job.PublishedAt != nil && !job.PublishedAt.IsZero() {
			jobHTML, err = renderTemplate("careers_job_unavailable", careersUnavailableJobTemplate, careersUnavailableJobPageData{
				Site:         bundle.Site,
				Job:          job,
				OpenJobs:     openJobs,
				HasCustomCSS: bundle.Site.CustomCSSEnabled && strings.TrimSpace(bundle.Site.CustomCSS) != "",
				PageURL:      absoluteURL(bundle.Site.WebsiteURL, "/careers/jobs/"+job.Slug),
			})
		} else {
			continue
		}
		if err != nil {
			return nil, err
		}
		files["site/jobs/"+job.Slug] = jobHTML
	}

	return files, nil
}

func renderTemplate(name, source string, data any) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"paragraphs":       paragraphs,
		"employmentLabel":  employmentLabel,
		"workModeLabel":    workModeLabel,
		"jobLink":          func(job Vacancy) string { return "/careers/jobs/" + job.Slug },
		"jobPublishedDate": jobPublishedDate,
		"jobStatusLabel":   jobStatusLabel,
		"locationLabel":    locationLabel,
		"hasContent": func(value string) bool {
			return strings.TrimSpace(value) != ""
		},
		"nonBlank": nonBlank,
	}).Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", name, err)
	}
	return buffer.Bytes(), nil
}

func filterOpenJobs(jobs []Vacancy) []Vacancy {
	filtered := make([]Vacancy, 0, len(jobs))
	for _, job := range jobs {
		if job.IsListedPublicly() && job.Status == atsdomain.VacancyStatusOpen {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func generalApplicationJob(jobs []Vacancy) *Vacancy {
	for _, job := range jobs {
		if job.Kind == atsdomain.VacancyKindGeneralApplication && job.Status == atsdomain.VacancyStatusOpen {
			jobCopy := job
			return &jobCopy
		}
	}
	return nil
}

func filterGallery(items []CareersGalleryItem, section string) []CareersGalleryItem {
	section = strings.TrimSpace(strings.ToLower(section))
	filtered := make([]CareersGalleryItem, 0, len(items))
	for _, item := range items {
		itemSection := strings.TrimSpace(strings.ToLower(item.Section))
		if itemSection == "" || itemSection == "all" || itemSection == section {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func paragraphs(value string) []string {
	chunks := strings.Split(strings.TrimSpace(value), "\n")
	result := make([]string, 0, len(chunks))
	var current []string
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			if len(current) > 0 {
				result = append(result, strings.Join(current, " "))
				current = nil
			}
			continue
		}
		current = append(current, chunk)
	}
	if len(current) > 0 {
		result = append(result, strings.Join(current, " "))
	}
	return result
}

func employmentLabel(value atsdomain.EmploymentType) string {
	switch value {
	case atsdomain.EmploymentTypeFullTime:
		return "Full-time"
	case atsdomain.EmploymentTypePartTime:
		return "Part-time"
	case atsdomain.EmploymentTypeContract:
		return "Contract"
	case atsdomain.EmploymentTypeInternship:
		return "Internship"
	default:
		return "Role"
	}
}

func workModeLabel(value atsdomain.WorkMode) string {
	switch value {
	case atsdomain.WorkModeRemote:
		return "Remote"
	case atsdomain.WorkModeHybrid:
		return "Hybrid"
	case atsdomain.WorkModeOnsite:
		return "On-site"
	default:
		return ""
	}
}

func jobStatusLabel(value atsdomain.VacancyStatus) string {
	switch value {
	case atsdomain.VacancyStatusOpen:
		return "Applications open"
	case atsdomain.VacancyStatusPaused:
		return "Hiring paused"
	case atsdomain.VacancyStatusClosed:
		return "Role closed"
	default:
		return "Draft"
	}
}

func locationLabel(job Vacancy) string {
	if strings.TrimSpace(job.Location) == "" {
		return workModeLabel(job.WorkMode)
	}
	if label := workModeLabel(job.WorkMode); label != "" {
		return job.Location + " • " + label
	}
	return job.Location
}

func jobPublishedDate(job Vacancy) string {
	if job.PublishedAt != nil && !job.PublishedAt.IsZero() {
		return job.PublishedAt.UTC().Format("2 January 2006")
	}
	return job.CreatedAt.UTC().Format("2 January 2006")
}

func nonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func marshalJSONLD(value any) template.JS {
	encoded, err := json.Marshal(value)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(encoded)
}

func buildOrganizationJSONLD(site CareersSiteProfile) map[string]any {
	address := postalAddress(site)
	payload := map[string]any{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     nonBlank(site.CompanyName, "Company"),
		"url":      site.WebsiteURL,
		"email":    site.ContactEmail,
	}
	if site.LogoURL != "" {
		payload["logo"] = site.LogoURL
	}
	if len(address) > 0 {
		payload["address"] = address
	}
	return payload
}

func buildJobPostingJSONLD(site CareersSiteProfile, job Vacancy) map[string]any {
	descriptionParts := []string{
		job.Summary,
		job.Description,
		job.AboutTheJob,
		strings.Join(job.Responsibilities, "; "),
		job.AboutYou,
		strings.Join(job.Profile, "; "),
		job.OffersIntro,
		strings.Join(job.Offers, "; "),
	}
	payload := map[string]any{
		"@context":           "https://schema.org",
		"@type":              "JobPosting",
		"title":              job.Title,
		"description":        strings.TrimSpace(strings.Join(filterEmptyStrings(descriptionParts), "\n\n")),
		"datePosted":         jobPostingDate(job),
		"employmentType":     schemaEmploymentType(job.EmploymentType),
		"hiringOrganization": buildOrganizationJSONLD(site),
		"url":                absoluteURL(site.WebsiteURL, "/careers/jobs/"+job.Slug),
	}
	if address := postalAddress(site); len(address) > 0 {
		payload["jobLocation"] = map[string]any{
			"@type":   "Place",
			"address": address,
		}
	}
	if job.WorkMode == atsdomain.WorkModeRemote {
		payload["jobLocationType"] = "TELECOMMUTE"
	}
	return payload
}

func absoluteURL(base, path string) string {
	base = strings.TrimSpace(base)
	path = strings.TrimSpace(path)
	if path == "" {
		return base
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func postalAddress(site CareersSiteProfile) map[string]any {
	if nonBlank(site.StreetAddress, site.AddressLocality, site.AddressRegion, site.PostalCode, site.AddressCountry) == "" {
		return nil
	}
	return map[string]any{
		"@type":           "PostalAddress",
		"streetAddress":   site.StreetAddress,
		"addressLocality": site.AddressLocality,
		"addressRegion":   site.AddressRegion,
		"postalCode":      site.PostalCode,
		"addressCountry":  site.AddressCountry,
	}
}

func jobPostingDate(job Vacancy) string {
	if job.PublishedAt != nil && !job.PublishedAt.IsZero() {
		return job.PublishedAt.UTC().Format(time.RFC3339)
	}
	return job.CreatedAt.UTC().Format(time.RFC3339)
}

func schemaEmploymentType(value atsdomain.EmploymentType) string {
	switch value {
	case atsdomain.EmploymentTypeFullTime:
		return "FULL_TIME"
	case atsdomain.EmploymentTypePartTime:
		return "PART_TIME"
	case atsdomain.EmploymentTypeContract:
		return "CONTRACTOR"
	case atsdomain.EmploymentTypeInternship:
		return "INTERN"
	default:
		return "FULL_TIME"
	}
}

func filterEmptyStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, strings.TrimSpace(value))
		}
	}
	return filtered
}

const careersHomeTemplate = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ nonBlank .Site.SiteTitle "Careers" }}</title>
    <meta name="description" content="{{ nonBlank .Site.MetaDescription .Site.Tagline }}">
    {{ if .PageURL }}<link rel="canonical" href="{{ .PageURL }}">{{ end }}
    <meta property="og:title" content="{{ nonBlank .Site.SiteTitle "Careers" }}">
    <meta property="og:description" content="{{ nonBlank .Site.MetaDescription .Site.Tagline }}">
    <meta property="og:type" content="website">
    {{ if .PageURL }}<meta property="og:url" content="{{ .PageURL }}">{{ end }}
    {{ if .Site.OgImageURL }}<meta property="og:image" content="{{ .Site.OgImageURL }}">{{ end }}
    <meta name="twitter:card" content="summary_large_image">
    <link rel="stylesheet" href="/careers/assets/site.css">
    {{ if .HasCustomCSS }}<link rel="stylesheet" href="/careers/assets/custom.css">{{ end }}
    <script defer src="/careers/assets/site.js"></script>
    <script type="application/ld+json">{{ .OrganizationJSON }}</script>
  </head>
  <body>
    <header class="site-shell">
      <div class="site-nav">
        <a class="brand" href="/careers">
          {{ if .Site.LogoURL }}<img src="{{ .Site.LogoURL }}" alt="{{ .Site.CompanyName }} logo">{{ end }}
          <span>{{ .Site.CompanyName }}</span>
        </a>
        <nav class="site-links">
          <a href="#open-roles">Roles</a>
          {{ if .GeneralApplyJob }}<a href="#general-application">General application</a>{{ end }}
          <a href="#team">Team</a>
          <a href="#gallery">Gallery</a>
        </nav>
      </div>
      <section class="hero-grid">
        <div class="hero-copy">
          <div class="eyebrow">{{ .Site.HeroEyebrow }}</div>
          <h1>{{ .Site.HeroTitle }}</h1>
          <p class="lead">{{ .Site.HeroBody }}</p>
          <div class="hero-actions">
            <a class="button-primary" href="{{ nonBlank .Site.HeroPrimaryHref "#open-roles" }}">{{ nonBlank .Site.HeroPrimaryLabel "See open roles" }}</a>
            <a class="button-secondary" href="{{ nonBlank .Site.HeroSecondaryHref "#team" }}">{{ nonBlank .Site.HeroSecondaryLabel "Meet the team" }}</a>
          </div>
        </div>
        <div class="hero-media">
          {{ if .Site.HeroImageURL }}
          <img src="{{ .Site.HeroImageURL }}" alt="{{ .Site.CompanyName }}">
          {{ else }}
          <div class="media-placeholder">Intentional systems. Calm execution.</div>
          {{ end }}
        </div>
      </section>
    </header>

    <main class="site-shell">
      <section class="story-block">
        <div class="section-heading">{{ .Site.StoryHeading }}</div>
        {{ range paragraphs .Site.StoryBody }}<p>{{ . }}</p>{{ end }}
      </section>

      <section id="open-roles" class="section-block">
        <div class="section-head">
          <div>
            <div class="section-heading">{{ .Site.JobsHeading }}</div>
            <p>{{ .Site.JobsIntro }}</p>
          </div>
          <div class="section-chip">{{ len .OpenJobs }} open roles</div>
        </div>
        <div class="jobs-grid">
          {{ if .OpenJobs }}
            {{ range .OpenJobs }}
            <article class="job-card">
              <div class="job-card-top">
                <div>
                  <h2><a href="{{ jobLink . }}">{{ .Title }}</a></h2>
                  <p>{{ .Summary }}</p>
                </div>
                <span class="status-pill">{{ jobStatusLabel .Status }}</span>
              </div>
              <div class="job-meta">
                <span>{{ .Team }}</span>
                <span>{{ locationLabel . }}</span>
                <span>{{ employmentLabel .EmploymentType }}</span>
              </div>
              <a class="text-link" href="{{ jobLink . }}">Read the role</a>
            </article>
            {{ end }}
          {{ else }}
            <div class="empty-state">
              <h3>We are not actively hiring right now.</h3>
              <p>Check back soon. We open roles when we can give them real attention.</p>
            </div>
          {{ end }}
        </div>
      </section>

      {{ if .GeneralApplyJob }}
      <section id="general-application" class="section-block">
        <div class="section-head">
          <div>
            <div class="section-heading">General application</div>
            <p>{{ nonBlank .GeneralApplyJob.Summary "Don’t see the exact role yet? Send a thoughtful application and we’ll route it well." }}</p>
          </div>
          <div class="section-chip">Open intake</div>
        </div>
        <div class="job-layout">
          <section class="copy-block">
            {{ range paragraphs (nonBlank .GeneralApplyJob.Description .GeneralApplyJob.AboutTheJob) }}<p>{{ . }}</p>{{ end }}
          </section>
          <aside class="job-sidebar">
            <section class="apply-card">
              <div class="section-heading">Introduce yourself</div>
              <p>Share the essentials and we’ll route your application into the right queue.</p>
              <form method="post" action="/careers/applications" data-ats-apply data-upload-enabled="{{ .ResumeUploadsEnabled }}">
                <input type="hidden" name="vacancySlug" value="{{ .GeneralApplyJob.Slug }}">
                <input type="hidden" name="resumeAttachmentId" value="">
                <label>Full name<input type="text" name="fullName" required></label>
                <label>Email<input type="email" name="email" required></label>
                <label>Phone<input type="text" name="phone"></label>
                <label>Location<input type="text" name="location"></label>
                <label>LinkedIn URL<input type="url" name="linkedinUrl"></label>
                <label>Portfolio URL<input type="url" name="portfolioUrl"></label>
                <label>What should we know?<textarea name="coverNote" rows="5"></textarea></label>
                <label class="file-field {{ if not .ResumeUploadsEnabled }}file-field-disabled{{ end }}">
                  Resume or CV
                  <input type="file" name="resumeFile" {{ if not .ResumeUploadsEnabled }}disabled{{ end }}>
                  {{ if not .ResumeUploadsEnabled }}<span class="help-text">File upload is not configured in this environment yet.</span>{{ end }}
                </label>
                <div class="form-feedback" data-feedback></div>
                <button class="button-primary button-full" type="submit">Send general application</button>
              </form>
            </section>
          </aside>
        </div>
      </section>
      {{ end }}

      <section id="team" class="section-block">
        <div class="section-head">
          <div>
            <div class="section-heading">{{ .Site.TeamHeading }}</div>
            <p>{{ .Site.TeamIntro }}</p>
          </div>
        </div>
        <div class="team-grid">
          {{ range .Team }}
          <article class="team-card">
            <div class="avatar">
              {{ if .ImageURL }}<img src="{{ .ImageURL }}" alt="{{ .Name }}">{{ else }}<span>{{ .Name }}</span>{{ end }}
            </div>
            <h3>{{ .Name }}</h3>
            <div class="team-role">{{ .Role }}</div>
            {{ if .Bio }}<p>{{ .Bio }}</p>{{ end }}
            {{ if .LinkedInURL }}<a class="text-link" href="{{ .LinkedInURL }}">LinkedIn</a>{{ end }}
          </article>
          {{ end }}
        </div>
      </section>

      <section id="gallery" class="section-block">
        <div class="section-head">
          <div>
            <div class="section-heading">{{ .Site.GalleryHeading }}</div>
            <p>{{ .Site.GalleryIntro }}</p>
          </div>
        </div>
        <div class="gallery-grid">
          {{ range .Gallery }}
          <figure class="gallery-card">
            {{ if .ImageURL }}
            <img src="{{ .ImageURL }}" alt="{{ nonBlank .AltText .Caption }}">
            {{ else }}
            <div class="gallery-placeholder">{{ nonBlank .AltText "Work in progress" }}</div>
            {{ end }}
            {{ if .Caption }}<figcaption>{{ .Caption }}</figcaption>{{ end }}
          </figure>
          {{ end }}
        </div>
      </section>
    </main>

    <footer class="site-shell site-footer">
      <div>
        <div class="section-heading">{{ .Site.CompanyName }}</div>
        <p>{{ .Site.Tagline }}</p>
      </div>
      <div class="footer-meta">
        {{ if .Site.ContactEmail }}<a href="mailto:{{ .Site.ContactEmail }}">{{ .Site.ContactEmail }}</a>{{ end }}
        {{ if .Site.WebsiteURL }}<a href="{{ .Site.WebsiteURL }}">Website</a>{{ end }}
        {{ if .Site.LinkedInURL }}<a href="{{ .Site.LinkedInURL }}">LinkedIn</a>{{ end }}
        {{ if .Site.PrivacyPolicyURL }}<a href="{{ .Site.PrivacyPolicyURL }}">Privacy</a>{{ end }}
      </div>
    </footer>
  </body>
</html>`

const careersJobTemplate = `<!doctype html>
<html lang="{{ nonBlank .Job.PublicLanguage "en" }}">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ .Job.Title }} · {{ .Site.CompanyName }}</title>
    <meta name="description" content="{{ nonBlank .Job.Summary .Site.MetaDescription }}">
    {{ if .PageURL }}<link rel="canonical" href="{{ .PageURL }}">{{ end }}
    <meta property="og:title" content="{{ .Job.Title }} · {{ .Site.CompanyName }}">
    <meta property="og:description" content="{{ nonBlank .Job.Summary .Site.MetaDescription }}">
    <meta property="og:type" content="website">
    {{ if .PageURL }}<meta property="og:url" content="{{ .PageURL }}">{{ end }}
    {{ if .Site.OgImageURL }}<meta property="og:image" content="{{ .Site.OgImageURL }}">{{ end }}
    <meta name="twitter:card" content="summary_large_image">
    <link rel="stylesheet" href="/careers/assets/site.css">
    {{ if .HasCustomCSS }}<link rel="stylesheet" href="/careers/assets/custom.css">{{ end }}
    <script defer src="/careers/assets/site.js"></script>
    <script type="application/ld+json">{{ .JobPostingJSON }}</script>
  </head>
  <body>
    <header class="site-shell">
      <div class="site-nav">
        <a class="brand" href="/careers">
          {{ if .Site.LogoURL }}<img src="{{ .Site.LogoURL }}" alt="{{ .Site.CompanyName }} logo">{{ end }}
          <span>{{ .Site.CompanyName }}</span>
        </a>
        <nav class="site-links">
          <a href="/careers#open-roles">All roles</a>
          <a href="#apply">Apply</a>
        </nav>
      </div>
      <section class="job-hero">
        <div>
          <div class="eyebrow">{{ .Job.Team }}</div>
          <h1>{{ .Job.Title }}</h1>
          <p class="lead">{{ .Job.Summary }}</p>
        </div>
        <div class="job-summary-card">
          <div class="summary-row"><span>Status</span><strong>{{ jobStatusLabel .Job.Status }}</strong></div>
          <div class="summary-row"><span>Location</span><strong>{{ locationLabel .Job }}</strong></div>
          <div class="summary-row"><span>Schedule</span><strong>{{ employmentLabel .Job.EmploymentType }}</strong></div>
          <div class="summary-row"><span>Posted</span><strong>{{ jobPublishedDate .Job }}</strong></div>
        </div>
      </section>
    </header>

    <main class="site-shell job-layout">
      <section class="job-content">
        {{ if hasContent .Job.AboutTheJob }}
        <section class="copy-block">
          <div class="section-heading">About the role</div>
          {{ range paragraphs .Job.AboutTheJob }}<p>{{ . }}</p>{{ end }}
        </section>
        {{ end }}

        {{ if .Job.Responsibilities }}
        <section class="copy-block">
          <div class="section-heading">{{ nonBlank .Job.ResponsibilitiesHeading "What you'll do" }}</div>
          <ul class="bullet-list">
            {{ range .Job.Responsibilities }}<li>{{ . }}</li>{{ end }}
          </ul>
        </section>
        {{ end }}

        {{ if hasContent .Job.AboutYou }}
        <section class="copy-block">
          <div class="section-heading">{{ nonBlank .Job.AboutYouHeading "About you" }}</div>
          {{ range paragraphs .Job.AboutYou }}<p>{{ . }}</p>{{ end }}
        </section>
        {{ end }}

        {{ if .Job.Profile }}
        <section class="copy-block">
          <div class="section-heading">What usually helps</div>
          <ul class="bullet-list">
            {{ range .Job.Profile }}<li>{{ . }}</li>{{ end }}
          </ul>
        </section>
        {{ end }}

        {{ if or (hasContent .Job.OffersIntro) .Job.Offers }}
        <section class="copy-block">
          <div class="section-heading">{{ nonBlank .Job.OffersHeading "What we offer" }}</div>
          {{ if hasContent .Job.OffersIntro }}<p>{{ .Job.OffersIntro }}</p>{{ end }}
          {{ if .Job.Offers }}
          <ul class="bullet-list">
            {{ range .Job.Offers }}<li>{{ . }}</li>{{ end }}
          </ul>
          {{ end }}
        </section>
        {{ end }}

        {{ if hasContent .Job.Quote }}
        <blockquote class="quote-block">“{{ .Job.Quote }}”</blockquote>
        {{ end }}

        {{ if .Gallery }}
        <section class="copy-block">
          <div class="section-heading">{{ .Site.GalleryHeading }}</div>
          <div class="gallery-grid gallery-grid-tight">
            {{ range .Gallery }}
            <figure class="gallery-card">
              {{ if .ImageURL }}
              <img src="{{ .ImageURL }}" alt="{{ nonBlank .AltText .Caption }}">
              {{ else }}
              <div class="gallery-placeholder">{{ nonBlank .AltText "How we work" }}</div>
              {{ end }}
              {{ if .Caption }}<figcaption>{{ .Caption }}</figcaption>{{ end }}
            </figure>
            {{ end }}
          </div>
        </section>
        {{ end }}
      </section>

      <aside class="job-sidebar" id="apply">
        {{ if .AcceptingApplications }}
        <section class="apply-card">
          <div class="section-heading">Apply for this role</div>
          <p>Share the essentials. We read every application carefully.</p>
          <form method="post" action="/careers/applications" data-ats-apply data-upload-enabled="{{ .ResumeUploadsEnabled }}">
            <input type="hidden" name="vacancySlug" value="{{ .Job.Slug }}">
            <input type="hidden" name="resumeAttachmentId" value="">
            <label>Full name<input type="text" name="fullName" required></label>
            <label>Email<input type="email" name="email" required></label>
            <label>Phone<input type="text" name="phone"></label>
            <label>Location<input type="text" name="location"></label>
            <label>LinkedIn URL<input type="url" name="linkedinUrl"></label>
            <label>Portfolio URL<input type="url" name="portfolioUrl"></label>
            <label>Why this role?<textarea name="coverNote" rows="5"></textarea></label>
            <label class="file-field {{ if not .ResumeUploadsEnabled }}file-field-disabled{{ end }}">
              Resume or CV
              <input type="file" name="resumeFile" {{ if not .ResumeUploadsEnabled }}disabled{{ end }}>
              {{ if not .ResumeUploadsEnabled }}<span class="help-text">File upload is not configured in this environment yet.</span>{{ end }}
            </label>
            <div class="form-feedback" data-feedback></div>
            <button class="button-primary button-full" type="submit">Send application</button>
          </form>
        </section>
        {{ else }}
        <section class="apply-card apply-card-muted">
          <div class="section-heading">{{ jobStatusLabel .Job.Status }}</div>
          <p>This role is no longer accepting applications. You can still review the position or explore other open roles.</p>
          <a class="button-secondary button-full" href="/careers#open-roles">See open roles</a>
        </section>
        {{ end }}

        {{ if .Team }}
        <section class="sidebar-card">
          <div class="section-heading">{{ .Site.TeamHeading }}</div>
          <div class="mini-team-list">
            {{ range .Team }}
            <article class="mini-team-member">
              <strong>{{ .Name }}</strong>
              <span>{{ .Role }}</span>
            </article>
            {{ end }}
          </div>
        </section>
        {{ end }}
      </aside>
    </main>
  </body>
</html>`

const careersUnavailableJobTemplate = `<!doctype html>
<html lang="{{ nonBlank .Job.PublicLanguage "en" }}">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ .Site.CompanyName }} Careers</title>
    <meta name="robots" content="noindex">
    {{ if .PageURL }}<link rel="canonical" href="{{ .PageURL }}">{{ end }}
    <link rel="stylesheet" href="/careers/assets/site.css">
    {{ if .HasCustomCSS }}<link rel="stylesheet" href="/careers/assets/custom.css">{{ end }}
  </head>
  <body>
    <header class="site-shell">
      <div class="site-nav">
        <a class="brand" href="/careers">
          {{ if .Site.LogoURL }}<img src="{{ .Site.LogoURL }}" alt="{{ .Site.CompanyName }} logo">{{ end }}
          <span>{{ .Site.CompanyName }}</span>
        </a>
      </div>
    </header>

    <main class="site-shell job-layout">
      <section class="copy-block">
        <div class="section-heading">{{ jobStatusLabel .Job.Status }}</div>
        <h1>This role is not currently published.</h1>
        <p>Draft, paused, and closed jobs are not shown as active public openings. You can browse the live careers site for roles that are currently open.</p>
        <div class="hero-actions">
          <a class="button-primary" href="/careers#open-roles">See open roles</a>
          {{ if .Site.ContactEmail }}<a class="button-secondary" href="mailto:{{ .Site.ContactEmail }}">Contact hiring</a>{{ end }}
        </div>
      </section>
    </main>
  </body>
</html>`

const careersApplyAliasTemplate = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta http-equiv="refresh" content="0; url=/careers#general-application">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ .SiteTitle }}</title>
    <link rel="stylesheet" href="/careers/assets/site.css">
  </head>
  <body>
    <main class="site-shell alias-page">
      <div class="apply-card">
        <div class="section-heading">Redirecting…</div>
        <p>Applications now live on role pages and through the general application section.</p>
        <a class="button-primary" href="/careers#general-application">Go to careers</a>
      </div>
    </main>
  </body>
</html>`

const careersStylesTemplate = `:root {
  --primary: {{ .PrimaryColor }};
  --accent: {{ .AccentColor }};
  --surface: {{ .SurfaceColor }};
  --background: {{ .BackgroundColor }};
  --text: {{ .TextColor }};
  --muted: {{ .MutedColor }};
  --border: rgba(18, 33, 27, 0.12);
  --shadow: 0 24px 60px rgba(18, 33, 27, 0.08);
}

* { box-sizing: border-box; }
html { scroll-behavior: smooth; }
body {
  margin: 0;
  font-family: Georgia, "Iowan Old Style", "Palatino Linotype", serif;
  background:
    radial-gradient(circle at top right, rgba(245, 158, 11, 0.18), transparent 30%),
    linear-gradient(180deg, #ffffff 0%, var(--background) 40%, #fffef9 100%);
  color: var(--text);
}
a { color: inherit; text-decoration: none; }
img { max-width: 100%; display: block; }
p { margin: 0 0 1rem; color: var(--muted); line-height: 1.7; }
h1, h2, h3 { margin: 0; line-height: 1.08; letter-spacing: -0.03em; }
h1 { font-size: clamp(3rem, 6vw, 5.4rem); max-width: 12ch; }
h2 { font-size: clamp(1.5rem, 2vw, 2rem); margin-bottom: 0.75rem; }
h3 { font-size: 1.2rem; margin-bottom: 0.4rem; }
label { display: block; font-size: 0.92rem; color: var(--muted); }
input, textarea {
  width: 100%;
  margin-top: 0.45rem;
  margin-bottom: 1rem;
  padding: 0.95rem 1rem;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: white;
  color: var(--text);
  font: inherit;
}
textarea { resize: vertical; min-height: 128px; }
.site-shell { width: min(1180px, calc(100vw - 2rem)); margin: 0 auto; }
.site-nav {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  padding: 1.25rem 0 1.5rem;
}
.brand { display: inline-flex; align-items: center; gap: 0.8rem; font-weight: 700; font-size: 1.05rem; }
.brand img { width: 44px; height: 44px; object-fit: cover; border-radius: 14px; }
.site-links { display: flex; gap: 1.1rem; color: var(--muted); font-size: 0.95rem; }
.hero-grid, .job-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(280px, 0.7fr);
  gap: 2rem;
  align-items: stretch;
  padding: 2rem 0 4rem;
}
.hero-copy, .hero-media, .job-summary-card, .story-block, .job-card, .team-card, .gallery-card, .apply-card, .sidebar-card {
  background: rgba(255,255,255,0.72);
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255,255,255,0.65);
  box-shadow: var(--shadow);
}
.hero-copy, .job-summary-card, .story-block, .job-card, .team-card, .apply-card, .sidebar-card { border-radius: 28px; padding: 1.8rem; }
.hero-media, .gallery-card { border-radius: 30px; overflow: hidden; }
.hero-media img, .gallery-card img { width: 100%; height: 100%; object-fit: cover; min-height: 100%; }
.media-placeholder, .gallery-placeholder {
  min-height: 100%;
  display: grid;
  place-items: center;
  padding: 2rem;
  text-align: center;
  color: white;
  background:
    linear-gradient(135deg, rgba(15, 118, 110, 0.92), rgba(18, 33, 27, 0.88)),
    radial-gradient(circle at top, rgba(245, 158, 11, 0.32), transparent 55%);
}
.media-placeholder { min-height: 420px; font-size: 1.5rem; font-weight: 600; }
.eyebrow, .section-heading { text-transform: uppercase; letter-spacing: 0.16em; font-size: 0.76rem; color: var(--primary); margin-bottom: 1rem; font-weight: 700; }
.lead { font-size: 1.1rem; max-width: 58ch; }
.hero-actions, .footer-meta, .job-meta, .site-links, .summary-row { display: flex; flex-wrap: wrap; }
.hero-actions { gap: 0.9rem; margin-top: 1.5rem; }
.button-primary, .button-secondary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 48px;
  padding: 0.95rem 1.2rem;
  border-radius: 999px;
  font-weight: 700;
}
.button-primary { background: var(--text); color: white; }
.button-secondary { border: 1px solid var(--border); background: rgba(255,255,255,0.7); }
.button-full { width: 100%; }
.section-block { padding: 1.25rem 0 0; }
.section-head {
  display: flex;
  justify-content: space-between;
  gap: 1.2rem;
  align-items: end;
  margin-bottom: 1.2rem;
}
.section-chip, .status-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  border-radius: 999px;
  padding: 0.55rem 0.85rem;
  background: rgba(15, 118, 110, 0.1);
  color: var(--primary);
  font-size: 0.85rem;
  font-weight: 700;
}
.jobs-grid, .team-grid, .gallery-grid {
  display: grid;
  gap: 1.1rem;
}
.jobs-grid { grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
.team-grid { grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
.gallery-grid { grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
.gallery-grid-tight { grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); }
.job-card-top { display: flex; justify-content: space-between; gap: 1rem; align-items: start; margin-bottom: 1rem; }
.job-meta { gap: 0.85rem; color: var(--muted); font-size: 0.92rem; margin-bottom: 1rem; }
.text-link { color: var(--primary); font-weight: 700; }
.avatar {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  overflow: hidden;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, rgba(15, 118, 110, 0.16), rgba(245, 158, 11, 0.18));
  margin-bottom: 1rem;
}
.avatar span { font-size: 0.9rem; font-weight: 700; padding: 0 0.5rem; text-align: center; }
.team-role, figcaption, .help-text, .form-feedback { color: var(--muted); font-size: 0.92rem; }
.site-footer {
  display: flex;
  justify-content: space-between;
  gap: 2rem;
  padding: 3rem 0 4rem;
}
.footer-meta { gap: 1rem; align-items: center; }
.job-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 380px);
  gap: 1.2rem;
  padding-bottom: 4rem;
}
.copy-block { margin-bottom: 2rem; }
.bullet-list { margin: 0; padding-left: 1.1rem; color: var(--muted); }
.bullet-list li { margin-bottom: 0.8rem; line-height: 1.65; }
.quote-block {
  margin: 2rem 0;
  padding: 1.6rem 1.8rem;
  border-left: 4px solid var(--accent);
  background: rgba(245, 158, 11, 0.08);
  border-radius: 20px;
  font-size: 1.25rem;
}
.summary-row {
  justify-content: space-between;
  gap: 1rem;
  padding: 0.8rem 0;
  border-bottom: 1px solid var(--border);
}
.summary-row:last-child { border-bottom: 0; }
.apply-card, .sidebar-card { position: sticky; top: 1rem; }
.apply-card-muted { background: rgba(255,255,255,0.62); }
.form-feedback { min-height: 1.25rem; margin-bottom: 0.75rem; }
.form-feedback[data-state="success"] { color: var(--primary); }
.form-feedback[data-state="error"] { color: #b91c1c; }
.file-field-disabled { opacity: 0.72; }
.mini-team-list { display: grid; gap: 0.8rem; }
.mini-team-member { display: grid; gap: 0.1rem; }
.empty-state, .alias-page { padding: 3rem 0 4rem; }

@media (max-width: 920px) {
  .hero-grid, .job-hero, .job-layout, .site-footer { grid-template-columns: 1fr; }
  .site-nav, .section-head { align-items: start; flex-direction: column; }
  .apply-card, .sidebar-card { position: static; }
}
`

const careersSiteJS = `(function() {
  function setFeedback(node, message, state) {
    if (!node) return;
    node.textContent = message || "";
    if (state) {
      node.setAttribute("data-state", state);
    } else {
      node.removeAttribute("data-state");
    }
  }

  async function uploadResume(form, file) {
    var payload = new FormData();
    payload.append("file", file);
    var response = await fetch("/careers/attachments", { method: "POST", body: payload });
    var data = await response.json().catch(function() { return {}; });
    if (!response.ok) {
      throw new Error(data.error || "Resume upload failed.");
    }
    return data.id || "";
  }

  async function submitApplication(form, resumeAttachmentId) {
    var payload = new FormData(form);
    if (resumeAttachmentId) {
      payload.set("resumeAttachmentId", resumeAttachmentId);
    }
    var response = await fetch(form.getAttribute("action") || "/careers/applications", {
      method: "POST",
      body: payload
    });
    var data = await response.json().catch(function() { return {}; });
    if (!response.ok) {
      throw new Error(data.error || "Application submission failed.");
    }
    return data;
  }

  document.querySelectorAll("form[data-ats-apply]").forEach(function(form) {
    form.addEventListener("submit", async function(event) {
      event.preventDefault();
      var feedback = form.querySelector("[data-feedback]");
      var submitButton = form.querySelector('button[type="submit"]');
      var fileInput = form.querySelector('input[type="file"]');
      var hiddenAttachment = form.querySelector('input[name="resumeAttachmentId"]');
      var uploadsEnabled = form.getAttribute("data-upload-enabled") === "true";

      submitButton.disabled = true;
      setFeedback(feedback, "Sending application…", "");

      try {
        var attachmentId = hiddenAttachment && hiddenAttachment.value ? hiddenAttachment.value : "";
        if (!attachmentId && uploadsEnabled && fileInput && fileInput.files && fileInput.files[0]) {
          setFeedback(feedback, "Uploading resume…", "");
          attachmentId = await uploadResume(form, fileInput.files[0]);
          if (hiddenAttachment) {
            hiddenAttachment.value = attachmentId;
          }
        }
        await submitApplication(form, attachmentId);
        form.reset();
        if (hiddenAttachment) {
          hiddenAttachment.value = "";
        }
        setFeedback(feedback, "Application received. We’ll follow up by email.", "success");
      } catch (error) {
        setFeedback(feedback, error && error.message ? error.message : "Could not submit the application.", "error");
      } finally {
        submitButton.disabled = false;
      }
    });
  });
})();`
