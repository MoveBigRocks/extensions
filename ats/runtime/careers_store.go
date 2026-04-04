package atsruntime

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func (s *Store) GetCareersSiteProfile(ctx context.Context, workspaceID string) (*CareersSiteProfile, error) {
	profile := &CareersSiteProfile{}
	if err := s.db.Get(ctx).GetContext(ctx, profile, s.query(`
		SELECT workspace_id, company_name, site_title, tagline, meta_description, hero_eyebrow,
			hero_title, hero_body, hero_primary_label, hero_primary_href, hero_secondary_label,
			hero_secondary_href, story_heading, story_body, jobs_heading, jobs_intro, team_heading,
			team_intro, gallery_heading, gallery_intro, contact_email, website_url, linkedin_url,
			instagram_url, x_url, logo_url, hero_image_url, og_image_url, primary_color, accent_color,
			surface_color, background_color, text_color, muted_color, street_address, address_locality,
			address_region, postal_code, address_country, privacy_policy_url, custom_css,
			custom_css_enabled, published_at, created_at, updated_at
		FROM ${SCHEMA_NAME}.careers_site_profiles
		WHERE workspace_id = ?
	`), strings.TrimSpace(workspaceID)); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("careers site profile not found")
		}
		return nil, fmt.Errorf("get careers site profile: %w", err)
	}
	return profile, nil
}

func (s *Store) SaveCareersSiteProfile(ctx context.Context, input UpsertCareersSiteInput) (*CareersSiteProfile, error) {
	profile := normalizeCareersSiteProfile(CareersSiteProfile{
		WorkspaceID:        input.WorkspaceID,
		CompanyName:        input.CompanyName,
		SiteTitle:          input.SiteTitle,
		Tagline:            input.Tagline,
		MetaDescription:    input.MetaDescription,
		HeroEyebrow:        input.HeroEyebrow,
		HeroTitle:          input.HeroTitle,
		HeroBody:           input.HeroBody,
		HeroPrimaryLabel:   input.HeroPrimaryLabel,
		HeroPrimaryHref:    input.HeroPrimaryHref,
		HeroSecondaryLabel: input.HeroSecondaryLabel,
		HeroSecondaryHref:  input.HeroSecondaryHref,
		StoryHeading:       input.StoryHeading,
		StoryBody:          input.StoryBody,
		JobsHeading:        input.JobsHeading,
		JobsIntro:          input.JobsIntro,
		TeamHeading:        input.TeamHeading,
		TeamIntro:          input.TeamIntro,
		GalleryHeading:     input.GalleryHeading,
		GalleryIntro:       input.GalleryIntro,
		ContactEmail:       input.ContactEmail,
		WebsiteURL:         input.WebsiteURL,
		LinkedInURL:        input.LinkedInURL,
		InstagramURL:       input.InstagramURL,
		XURL:               input.XURL,
		LogoURL:            input.LogoURL,
		HeroImageURL:       input.HeroImageURL,
		OgImageURL:         input.OgImageURL,
		PrimaryColor:       input.PrimaryColor,
		AccentColor:        input.AccentColor,
		SurfaceColor:       input.SurfaceColor,
		BackgroundColor:    input.BackgroundColor,
		TextColor:          input.TextColor,
		MutedColor:         input.MutedColor,
		StreetAddress:      input.StreetAddress,
		AddressLocality:    input.AddressLocality,
		AddressRegion:      input.AddressRegion,
		PostalCode:         input.PostalCode,
		AddressCountry:     input.AddressCountry,
		PrivacyPolicyURL:   input.PrivacyPolicyURL,
		CustomCSS:          input.CustomCSS,
		CustomCSSEnabled:   input.CustomCSSEnabled,
	})
	now := time.Now().UTC()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	saved := &CareersSiteProfile{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		INSERT INTO ${SCHEMA_NAME}.careers_site_profiles (
			workspace_id, company_name, site_title, tagline, meta_description, hero_eyebrow,
			hero_title, hero_body, hero_primary_label, hero_primary_href, hero_secondary_label,
			hero_secondary_href, story_heading, story_body, jobs_heading, jobs_intro, team_heading,
			team_intro, gallery_heading, gallery_intro, contact_email, website_url, linkedin_url,
			instagram_url, x_url, logo_url, hero_image_url, og_image_url, primary_color, accent_color,
			surface_color, background_color, text_color, muted_color, street_address, address_locality,
			address_region, postal_code, address_country, privacy_policy_url, custom_css,
			custom_css_enabled, published_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id) DO UPDATE
		SET company_name = EXCLUDED.company_name,
			site_title = EXCLUDED.site_title,
			tagline = EXCLUDED.tagline,
			meta_description = EXCLUDED.meta_description,
			hero_eyebrow = EXCLUDED.hero_eyebrow,
			hero_title = EXCLUDED.hero_title,
			hero_body = EXCLUDED.hero_body,
			hero_primary_label = EXCLUDED.hero_primary_label,
			hero_primary_href = EXCLUDED.hero_primary_href,
			hero_secondary_label = EXCLUDED.hero_secondary_label,
			hero_secondary_href = EXCLUDED.hero_secondary_href,
			story_heading = EXCLUDED.story_heading,
			story_body = EXCLUDED.story_body,
			jobs_heading = EXCLUDED.jobs_heading,
			jobs_intro = EXCLUDED.jobs_intro,
			team_heading = EXCLUDED.team_heading,
			team_intro = EXCLUDED.team_intro,
			gallery_heading = EXCLUDED.gallery_heading,
			gallery_intro = EXCLUDED.gallery_intro,
			contact_email = EXCLUDED.contact_email,
			website_url = EXCLUDED.website_url,
			linkedin_url = EXCLUDED.linkedin_url,
			instagram_url = EXCLUDED.instagram_url,
			x_url = EXCLUDED.x_url,
			logo_url = EXCLUDED.logo_url,
			hero_image_url = EXCLUDED.hero_image_url,
			og_image_url = EXCLUDED.og_image_url,
			primary_color = EXCLUDED.primary_color,
			accent_color = EXCLUDED.accent_color,
			surface_color = EXCLUDED.surface_color,
			background_color = EXCLUDED.background_color,
			text_color = EXCLUDED.text_color,
			muted_color = EXCLUDED.muted_color,
			street_address = EXCLUDED.street_address,
			address_locality = EXCLUDED.address_locality,
			address_region = EXCLUDED.address_region,
			postal_code = EXCLUDED.postal_code,
			address_country = EXCLUDED.address_country,
			privacy_policy_url = EXCLUDED.privacy_policy_url,
			custom_css = EXCLUDED.custom_css,
			custom_css_enabled = EXCLUDED.custom_css_enabled,
			published_at = COALESCE(${SCHEMA_NAME}.careers_site_profiles.published_at, EXCLUDED.published_at),
			updated_at = EXCLUDED.updated_at
		RETURNING workspace_id, company_name, site_title, tagline, meta_description, hero_eyebrow,
			hero_title, hero_body, hero_primary_label, hero_primary_href, hero_secondary_label,
			hero_secondary_href, story_heading, story_body, jobs_heading, jobs_intro, team_heading,
			team_intro, gallery_heading, gallery_intro, contact_email, website_url, linkedin_url,
			instagram_url, x_url, logo_url, hero_image_url, og_image_url, primary_color, accent_color,
			surface_color, background_color, text_color, muted_color, street_address, address_locality,
			address_region, postal_code, address_country, privacy_policy_url, custom_css,
			custom_css_enabled, published_at, created_at, updated_at
	`),
		profile.WorkspaceID,
		profile.CompanyName,
		profile.SiteTitle,
		profile.Tagline,
		profile.MetaDescription,
		profile.HeroEyebrow,
		profile.HeroTitle,
		profile.HeroBody,
		profile.HeroPrimaryLabel,
		profile.HeroPrimaryHref,
		profile.HeroSecondaryLabel,
		profile.HeroSecondaryHref,
		profile.StoryHeading,
		profile.StoryBody,
		profile.JobsHeading,
		profile.JobsIntro,
		profile.TeamHeading,
		profile.TeamIntro,
		profile.GalleryHeading,
		profile.GalleryIntro,
		profile.ContactEmail,
		profile.WebsiteURL,
		profile.LinkedInURL,
		profile.InstagramURL,
		profile.XURL,
		profile.LogoURL,
		profile.HeroImageURL,
		profile.OgImageURL,
		profile.PrimaryColor,
		profile.AccentColor,
		profile.SurfaceColor,
		profile.BackgroundColor,
		profile.TextColor,
		profile.MutedColor,
		profile.StreetAddress,
		profile.AddressLocality,
		profile.AddressRegion,
		profile.PostalCode,
		profile.AddressCountry,
		profile.PrivacyPolicyURL,
		profile.CustomCSS,
		profile.CustomCSSEnabled,
		profile.PublishedAt,
		profile.CreatedAt,
		profile.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("save careers site profile: %w", err)
	}
	return saved, nil
}

func (s *Store) MarkCareersSitePublished(ctx context.Context, workspaceID string, publishedAt time.Time) (*CareersSiteProfile, error) {
	profile := &CareersSiteProfile{}
	if err := s.db.Get(ctx).GetContext(ctx, profile, s.query(`
		UPDATE ${SCHEMA_NAME}.careers_site_profiles
		SET published_at = ?, updated_at = ?
		WHERE workspace_id = ?
		RETURNING workspace_id, company_name, site_title, tagline, meta_description, hero_eyebrow,
			hero_title, hero_body, hero_primary_label, hero_primary_href, hero_secondary_label,
			hero_secondary_href, story_heading, story_body, jobs_heading, jobs_intro, team_heading,
			team_intro, gallery_heading, gallery_intro, contact_email, website_url, linkedin_url,
			instagram_url, x_url, logo_url, hero_image_url, og_image_url, primary_color, accent_color,
			surface_color, background_color, text_color, muted_color, street_address, address_locality,
			address_region, postal_code, address_country, privacy_policy_url, custom_css,
			custom_css_enabled, published_at, created_at, updated_at
	`), publishedAt.UTC(), time.Now().UTC(), strings.TrimSpace(workspaceID)); err != nil {
		return nil, fmt.Errorf("mark careers site published: %w", err)
	}
	return profile, nil
}

func (s *Store) GetCareersSetupState(ctx context.Context, workspaceID string) (*CareersSetupState, error) {
	state := &CareersSetupState{}
	if err := s.db.Get(ctx).GetContext(ctx, state, s.query(`
		SELECT workspace_id, current_step, confirmed_steps, completed_at, created_at, updated_at
		FROM ${SCHEMA_NAME}.careers_setup_states
		WHERE workspace_id = ?
	`), strings.TrimSpace(workspaceID)); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("careers setup state not found")
		}
		return nil, fmt.Errorf("get careers setup state: %w", err)
	}
	return state, nil
}

func (s *Store) SaveCareersSetupState(ctx context.Context, state CareersSetupState) (*CareersSetupState, error) {
	state.WorkspaceID = strings.TrimSpace(state.WorkspaceID)
	state.CurrentStep = strings.TrimSpace(strings.ToLower(state.CurrentStep))
	state.ConfirmedSteps = normalizeSetupSteps(state.ConfirmedSteps)
	if state.CurrentStep == "" {
		state.CurrentStep = "brand"
	}
	now := time.Now().UTC()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now

	saved := &CareersSetupState{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		INSERT INTO ${SCHEMA_NAME}.careers_setup_states (
			workspace_id, current_step, confirmed_steps, completed_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id) DO UPDATE
		SET current_step = EXCLUDED.current_step,
			confirmed_steps = EXCLUDED.confirmed_steps,
			completed_at = EXCLUDED.completed_at,
			updated_at = EXCLUDED.updated_at
		RETURNING workspace_id, current_step, confirmed_steps, completed_at, created_at, updated_at
	`),
		state.WorkspaceID,
		state.CurrentStep,
		state.ConfirmedSteps,
		state.CompletedAt,
		state.CreatedAt,
		state.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("save careers setup state: %w", err)
	}
	return saved, nil
}

func (s *Store) ListCareersTeamMembers(ctx context.Context, workspaceID string) ([]CareersTeamMember, error) {
	var members []CareersTeamMember
	if err := s.db.Get(ctx).SelectContext(ctx, &members, s.query(`
		SELECT id, workspace_id, display_order, name, role, bio, image_url, linkedin_url, created_at, updated_at
		FROM ${SCHEMA_NAME}.careers_team_members
		WHERE workspace_id = ?
		ORDER BY display_order ASC, id ASC
	`), strings.TrimSpace(workspaceID)); err != nil {
		return nil, fmt.Errorf("list careers team members: %w", err)
	}
	return members, nil
}

func (s *Store) ReplaceCareersTeamMembers(ctx context.Context, workspaceID string, members []CareersTeamMember) ([]CareersTeamMember, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	now := time.Now().UTC()
	normalized := make([]CareersTeamMember, 0, len(members))
	for index, member := range members {
		member = normalizeCareersTeamMember(member)
		member.WorkspaceID = workspaceID
		member.DisplayOrder = index
		if strings.TrimSpace(member.ID) == "" {
			member.ID = uuid.NewString()
		}
		if member.CreatedAt.IsZero() {
			member.CreatedAt = now
		}
		member.UpdatedAt = now
		normalized = append(normalized, member)
	}
	if err := s.db.Transaction(ctx, func(txCtx context.Context) error {
		if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
			DELETE FROM ${SCHEMA_NAME}.careers_team_members WHERE workspace_id = ?
		`), workspaceID); err != nil {
			return fmt.Errorf("clear careers team members: %w", err)
		}
		for _, member := range normalized {
			if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
				INSERT INTO ${SCHEMA_NAME}.careers_team_members (
					id, workspace_id, display_order, name, role, bio, image_url, linkedin_url, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`),
				member.ID,
				member.WorkspaceID,
				member.DisplayOrder,
				member.Name,
				member.Role,
				member.Bio,
				member.ImageURL,
				member.LinkedInURL,
				member.CreatedAt,
				member.UpdatedAt,
			); err != nil {
				return fmt.Errorf("insert careers team member %s: %w", member.Name, err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.ListCareersTeamMembers(ctx, workspaceID)
}

func (s *Store) ListCareersGalleryItems(ctx context.Context, workspaceID string) ([]CareersGalleryItem, error) {
	var items []CareersGalleryItem
	if err := s.db.Get(ctx).SelectContext(ctx, &items, s.query(`
		SELECT id, workspace_id, display_order, section, alt_text, caption, image_url, created_at, updated_at
		FROM ${SCHEMA_NAME}.careers_gallery_items
		WHERE workspace_id = ?
		ORDER BY section ASC, display_order ASC, id ASC
	`), strings.TrimSpace(workspaceID)); err != nil {
		return nil, fmt.Errorf("list careers gallery items: %w", err)
	}
	return items, nil
}

func (s *Store) ReplaceCareersGalleryItems(ctx context.Context, workspaceID string, items []CareersGalleryItem) ([]CareersGalleryItem, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	now := time.Now().UTC()
	normalized := make([]CareersGalleryItem, 0, len(items))
	for index, item := range items {
		item = normalizeCareersGalleryItem(item)
		item.WorkspaceID = workspaceID
		item.DisplayOrder = index
		if strings.TrimSpace(item.ID) == "" {
			item.ID = uuid.NewString()
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		item.UpdatedAt = now
		normalized = append(normalized, item)
	}
	if err := s.db.Transaction(ctx, func(txCtx context.Context) error {
		if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
			DELETE FROM ${SCHEMA_NAME}.careers_gallery_items WHERE workspace_id = ?
		`), workspaceID); err != nil {
			return fmt.Errorf("clear careers gallery items: %w", err)
		}
		for _, item := range normalized {
			if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
				INSERT INTO ${SCHEMA_NAME}.careers_gallery_items (
					id, workspace_id, display_order, section, alt_text, caption, image_url, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`),
				item.ID,
				item.WorkspaceID,
				item.DisplayOrder,
				item.Section,
				item.AltText,
				item.Caption,
				item.ImageURL,
				item.CreatedAt,
				item.UpdatedAt,
			); err != nil {
				return fmt.Errorf("insert careers gallery item %s: %w", item.ID, err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.ListCareersGalleryItems(ctx, workspaceID)
}

func (s *Store) ListCareersMediaAssets(ctx context.Context, workspaceID string) ([]CareersMediaAsset, error) {
	var assets []CareersMediaAsset
	if err := s.db.Get(ctx).SelectContext(ctx, &assets, s.query(`
		SELECT id, workspace_id, purpose, filename, content_type, size_bytes, artifact_path, public_url, created_at, updated_at
		FROM ${SCHEMA_NAME}.careers_media_assets
		WHERE workspace_id = ?
		ORDER BY created_at DESC, id DESC
	`), strings.TrimSpace(workspaceID)); err != nil {
		return nil, fmt.Errorf("list careers media assets: %w", err)
	}
	return assets, nil
}

func (s *Store) SaveCareersMediaAsset(ctx context.Context, asset CareersMediaAsset) (*CareersMediaAsset, error) {
	asset.WorkspaceID = strings.TrimSpace(asset.WorkspaceID)
	asset.Purpose = normalizeMediaPurpose(asset.Purpose)
	asset.Filename = strings.TrimSpace(asset.Filename)
	asset.ContentType = strings.TrimSpace(asset.ContentType)
	asset.ArtifactPath = strings.TrimSpace(asset.ArtifactPath)
	asset.PublicURL = strings.TrimSpace(asset.PublicURL)
	if asset.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if asset.ID == "" {
		asset.ID = uuid.NewString()
	}
	if asset.ArtifactPath == "" {
		return nil, fmt.Errorf("artifact path is required")
	}
	now := time.Now().UTC()
	if asset.CreatedAt.IsZero() {
		asset.CreatedAt = now
	}
	asset.UpdatedAt = now
	saved := &CareersMediaAsset{}
	if err := s.db.Get(ctx).GetContext(ctx, saved, s.query(`
		INSERT INTO ${SCHEMA_NAME}.careers_media_assets (
			id, workspace_id, purpose, filename, content_type, size_bytes, artifact_path, public_url, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (workspace_id, artifact_path) DO UPDATE
		SET purpose = EXCLUDED.purpose,
			filename = EXCLUDED.filename,
			content_type = EXCLUDED.content_type,
			size_bytes = EXCLUDED.size_bytes,
			public_url = EXCLUDED.public_url,
			updated_at = EXCLUDED.updated_at
		RETURNING id, workspace_id, purpose, filename, content_type, size_bytes, artifact_path, public_url, created_at, updated_at
	`),
		asset.ID,
		asset.WorkspaceID,
		asset.Purpose,
		asset.Filename,
		asset.ContentType,
		asset.SizeBytes,
		asset.ArtifactPath,
		asset.PublicURL,
		asset.CreatedAt,
		asset.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("save careers media asset: %w", err)
	}
	return saved, nil
}

func (s *Store) ensureCareersSiteProfile(ctx context.Context, workspaceID string) error {
	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.careers_site_profiles WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return fmt.Errorf("count careers site profiles: %w", err)
	}
	if count > 0 {
		return nil
	}
	_, err := s.SaveCareersSiteProfile(ctx, defaultCareersSiteProfile(workspaceID))
	return err
}

func (s *Store) ensureCareersSetupState(ctx context.Context, workspaceID string) error {
	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.careers_setup_states WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return fmt.Errorf("count careers setup states: %w", err)
	}
	if count > 0 {
		return nil
	}
	_, err := s.SaveCareersSetupState(ctx, CareersSetupState{
		WorkspaceID: workspaceID,
		CurrentStep: "brand",
	})
	return err
}

func (s *Store) ensureCareersTeamMembers(ctx context.Context, workspaceID string) error {
	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.careers_team_members WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return fmt.Errorf("count careers team members: %w", err)
	}
	if count > 0 {
		return nil
	}
	now := time.Now().UTC()
	for index, member := range defaultCareersTeamMembers(workspaceID) {
		member = normalizeCareersTeamMember(member)
		member.WorkspaceID = workspaceID
		member.DisplayOrder = index
		member.ID = uuid.NewString()
		member.CreatedAt = now
		member.UpdatedAt = now
		if _, err := s.db.Get(ctx).ExecContext(ctx, s.query(`
			INSERT INTO ${SCHEMA_NAME}.careers_team_members (
				id, workspace_id, display_order, name, role, bio, image_url, linkedin_url, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`),
			member.ID,
			member.WorkspaceID,
			member.DisplayOrder,
			member.Name,
			member.Role,
			member.Bio,
			member.ImageURL,
			member.LinkedInURL,
			member.CreatedAt,
			member.UpdatedAt,
		); err != nil {
			return fmt.Errorf("seed careers team member %s: %w", member.Name, err)
		}
	}
	return nil
}

func (s *Store) ensureCareersGalleryItems(ctx context.Context, workspaceID string) error {
	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.careers_gallery_items WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return fmt.Errorf("count careers gallery items: %w", err)
	}
	if count > 0 {
		return nil
	}
	now := time.Now().UTC()
	for index, item := range defaultCareersGalleryItems(workspaceID) {
		item = normalizeCareersGalleryItem(item)
		item.WorkspaceID = workspaceID
		item.DisplayOrder = index
		item.ID = uuid.NewString()
		item.CreatedAt = now
		item.UpdatedAt = now
		if _, err := s.db.Get(ctx).ExecContext(ctx, s.query(`
			INSERT INTO ${SCHEMA_NAME}.careers_gallery_items (
				id, workspace_id, display_order, section, alt_text, caption, image_url, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`),
			item.ID,
			item.WorkspaceID,
			item.DisplayOrder,
			item.Section,
			item.AltText,
			item.Caption,
			item.ImageURL,
			item.CreatedAt,
			item.UpdatedAt,
		); err != nil {
			return fmt.Errorf("seed careers gallery item %s: %w", item.ID, err)
		}
	}
	return nil
}

func normalizeCareersSiteProfile(profile CareersSiteProfile) CareersSiteProfile {
	profile.WorkspaceID = strings.TrimSpace(profile.WorkspaceID)
	profile.CompanyName = strings.TrimSpace(profile.CompanyName)
	profile.SiteTitle = strings.TrimSpace(profile.SiteTitle)
	profile.Tagline = strings.TrimSpace(profile.Tagline)
	profile.MetaDescription = strings.TrimSpace(profile.MetaDescription)
	profile.HeroEyebrow = strings.TrimSpace(profile.HeroEyebrow)
	profile.HeroTitle = strings.TrimSpace(profile.HeroTitle)
	profile.HeroBody = strings.TrimSpace(profile.HeroBody)
	profile.HeroPrimaryLabel = strings.TrimSpace(profile.HeroPrimaryLabel)
	profile.HeroPrimaryHref = strings.TrimSpace(profile.HeroPrimaryHref)
	profile.HeroSecondaryLabel = strings.TrimSpace(profile.HeroSecondaryLabel)
	profile.HeroSecondaryHref = strings.TrimSpace(profile.HeroSecondaryHref)
	profile.StoryHeading = strings.TrimSpace(profile.StoryHeading)
	profile.StoryBody = strings.TrimSpace(profile.StoryBody)
	profile.JobsHeading = strings.TrimSpace(profile.JobsHeading)
	profile.JobsIntro = strings.TrimSpace(profile.JobsIntro)
	profile.TeamHeading = strings.TrimSpace(profile.TeamHeading)
	profile.TeamIntro = strings.TrimSpace(profile.TeamIntro)
	profile.GalleryHeading = strings.TrimSpace(profile.GalleryHeading)
	profile.GalleryIntro = strings.TrimSpace(profile.GalleryIntro)
	profile.ContactEmail = strings.TrimSpace(profile.ContactEmail)
	profile.WebsiteURL = strings.TrimSpace(profile.WebsiteURL)
	profile.LinkedInURL = strings.TrimSpace(profile.LinkedInURL)
	profile.InstagramURL = strings.TrimSpace(profile.InstagramURL)
	profile.XURL = strings.TrimSpace(profile.XURL)
	profile.LogoURL = strings.TrimSpace(profile.LogoURL)
	profile.HeroImageURL = strings.TrimSpace(profile.HeroImageURL)
	profile.OgImageURL = strings.TrimSpace(profile.OgImageURL)
	profile.PrimaryColor = strings.TrimSpace(profile.PrimaryColor)
	profile.AccentColor = strings.TrimSpace(profile.AccentColor)
	profile.SurfaceColor = strings.TrimSpace(profile.SurfaceColor)
	profile.BackgroundColor = strings.TrimSpace(profile.BackgroundColor)
	profile.TextColor = strings.TrimSpace(profile.TextColor)
	profile.MutedColor = strings.TrimSpace(profile.MutedColor)
	profile.StreetAddress = strings.TrimSpace(profile.StreetAddress)
	profile.AddressLocality = strings.TrimSpace(profile.AddressLocality)
	profile.AddressRegion = strings.TrimSpace(profile.AddressRegion)
	profile.PostalCode = strings.TrimSpace(profile.PostalCode)
	profile.AddressCountry = strings.TrimSpace(profile.AddressCountry)
	profile.PrivacyPolicyURL = strings.TrimSpace(profile.PrivacyPolicyURL)
	profile.CustomCSS = strings.TrimSpace(profile.CustomCSS)
	return profile
}

func normalizeCareersTeamMember(member CareersTeamMember) CareersTeamMember {
	member.Name = strings.TrimSpace(member.Name)
	member.Role = strings.TrimSpace(member.Role)
	member.Bio = strings.TrimSpace(member.Bio)
	member.ImageURL = strings.TrimSpace(member.ImageURL)
	member.LinkedInURL = strings.TrimSpace(member.LinkedInURL)
	return member
}

func normalizeCareersGalleryItem(item CareersGalleryItem) CareersGalleryItem {
	item.Section = strings.TrimSpace(strings.ToLower(item.Section))
	if item.Section == "" {
		item.Section = "homepage"
	}
	item.AltText = strings.TrimSpace(item.AltText)
	item.Caption = strings.TrimSpace(item.Caption)
	item.ImageURL = strings.TrimSpace(item.ImageURL)
	return item
}

func normalizeMediaPurpose(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "logo", "hero", "og", "team", "gallery", "brand", "other":
		return value
	default:
		return "other"
	}
}

func normalizeSetupSteps(steps []string) pq.StringArray {
	if len(steps) == 0 {
		return pq.StringArray{}
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(steps))
	for _, step := range steps {
		step = strings.TrimSpace(strings.ToLower(step))
		if step == "" {
			continue
		}
		if _, ok := seen[step]; ok {
			continue
		}
		seen[step] = struct{}{}
		normalized = append(normalized, step)
	}
	return pq.StringArray(normalized)
}

func defaultCareersSiteProfile(workspaceID string) UpsertCareersSiteInput {
	return UpsertCareersSiteInput{
		WorkspaceID:        workspaceID,
		CompanyName:        "Acme",
		SiteTitle:          "Careers at Acme",
		Tagline:            "Build calm systems that ambitious teams can trust.",
		MetaDescription:    "Join Acme and help shape the operating system for ambitious service teams.",
		HeroEyebrow:        "Thoughtful hiring, real ownership",
		HeroTitle:          "Build the systems teams rely on every day.",
		HeroBody:           "We are building a company where thoughtful operators and builders can do the best work of their careers. That means sharp problems, meaningful ownership, and a pace that stays ambitious without turning chaotic.",
		HeroPrimaryLabel:   "See open roles",
		HeroPrimaryHref:    "#open-roles",
		HeroSecondaryLabel: "Meet the team",
		HeroSecondaryHref:  "#team",
		StoryHeading:       "What we're building",
		StoryBody:          "Acme builds tools that help service, operations, and support teams move with more clarity. We care about systems that feel calm, durable, and deeply useful.\n\nWe hire for judgment, craft, and curiosity. If you like taking ownership and making the product feel sharper for real teams, you'll fit right in.",
		JobsHeading:        "Open roles",
		JobsIntro:          "We publish roles when we are ready to invest in them properly. Each opening has a clear scope, a real hiring manager, and a meaningful amount of ownership attached to it.",
		TeamHeading:        "The people you'll work with",
		TeamIntro:          "A small team of operators, designers, and engineers who like working closely, shipping carefully, and getting stronger together.",
		GalleryHeading:     "How we work",
		GalleryIntro:       "A few glimpses of the product, the craft, and the pace we optimize for.",
		ContactEmail:       "careers@acme.example",
		WebsiteURL:         "https://acme.example",
		LinkedInURL:        "https://linkedin.com/company/acme",
		PrimaryColor:       "#0f766e",
		AccentColor:        "#f59e0b",
		SurfaceColor:       "#f6f5f0",
		BackgroundColor:    "#fbfaf6",
		TextColor:          "#12211b",
		MutedColor:         "#5f6b65",
		StreetAddress:      "101 Market Street",
		AddressLocality:    "Amsterdam",
		AddressRegion:      "North Holland",
		PostalCode:         "1012 AB",
		AddressCountry:     "NL",
		PrivacyPolicyURL:   "https://acme.example/privacy",
	}
}

func defaultCareersTeamMembers(workspaceID string) []CareersTeamMember {
	return []CareersTeamMember{
		{WorkspaceID: workspaceID, Name: "Maya Chen", Role: "Head of Product", Bio: "Keeps the product honest by staying close to operators and the rough edges that still matter."},
		{WorkspaceID: workspaceID, Name: "Jonas de Vries", Role: "Founding Engineer", Bio: "Builds durable systems, sweats the details, and loves turning abstract product goals into clear software."},
		{WorkspaceID: workspaceID, Name: "Amina Rahman", Role: "Design Lead", Bio: "Shapes interfaces that feel clear under pressure and still have a point of view."},
	}
}

func defaultCareersGalleryItems(workspaceID string) []CareersGalleryItem {
	return []CareersGalleryItem{
		{WorkspaceID: workspaceID, Section: "homepage", AltText: "Design critique board", Caption: "Weekly design critiques keep the bar visible."},
		{WorkspaceID: workspaceID, Section: "homepage", AltText: "Planning session", Caption: "Strategy gets clearer when engineering and product shape it together."},
		{WorkspaceID: workspaceID, Section: "jobs", AltText: "Workspace setup", Caption: "The tools should feel as intentional as the outcomes."},
		{WorkspaceID: workspaceID, Section: "jobs", AltText: "Team offsite", Caption: "We invest in alignment because it compounds through the whole system."},
	}
}
