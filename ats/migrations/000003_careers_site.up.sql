CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.careers_site_profiles (
    workspace_id TEXT PRIMARY KEY,
    company_name TEXT NOT NULL DEFAULT '',
    site_title TEXT NOT NULL DEFAULT '',
    tagline TEXT NOT NULL DEFAULT '',
    meta_description TEXT NOT NULL DEFAULT '',
    hero_eyebrow TEXT NOT NULL DEFAULT '',
    hero_title TEXT NOT NULL DEFAULT '',
    hero_body TEXT NOT NULL DEFAULT '',
    hero_primary_label TEXT NOT NULL DEFAULT '',
    hero_primary_href TEXT NOT NULL DEFAULT '',
    hero_secondary_label TEXT NOT NULL DEFAULT '',
    hero_secondary_href TEXT NOT NULL DEFAULT '',
    story_heading TEXT NOT NULL DEFAULT '',
    story_body TEXT NOT NULL DEFAULT '',
    jobs_heading TEXT NOT NULL DEFAULT '',
    jobs_intro TEXT NOT NULL DEFAULT '',
    team_heading TEXT NOT NULL DEFAULT '',
    team_intro TEXT NOT NULL DEFAULT '',
    gallery_heading TEXT NOT NULL DEFAULT '',
    gallery_intro TEXT NOT NULL DEFAULT '',
    contact_email TEXT NOT NULL DEFAULT '',
    website_url TEXT NOT NULL DEFAULT '',
    linkedin_url TEXT NOT NULL DEFAULT '',
    instagram_url TEXT NOT NULL DEFAULT '',
    x_url TEXT NOT NULL DEFAULT '',
    logo_url TEXT NOT NULL DEFAULT '',
    hero_image_url TEXT NOT NULL DEFAULT '',
    og_image_url TEXT NOT NULL DEFAULT '',
    primary_color TEXT NOT NULL DEFAULT '',
    accent_color TEXT NOT NULL DEFAULT '',
    surface_color TEXT NOT NULL DEFAULT '',
    background_color TEXT NOT NULL DEFAULT '',
    text_color TEXT NOT NULL DEFAULT '',
    muted_color TEXT NOT NULL DEFAULT '',
    street_address TEXT NOT NULL DEFAULT '',
    address_locality TEXT NOT NULL DEFAULT '',
    address_region TEXT NOT NULL DEFAULT '',
    postal_code TEXT NOT NULL DEFAULT '',
    address_country TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.careers_team_members (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    linkedin_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ats_careers_team_members_workspace
    ON ${SCHEMA_NAME}.careers_team_members(workspace_id, display_order ASC, id ASC);

CREATE TABLE IF NOT EXISTS ${SCHEMA_NAME}.careers_gallery_items (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    section TEXT NOT NULL DEFAULT 'homepage',
    alt_text TEXT NOT NULL DEFAULT '',
    caption TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ats_careers_gallery_items_workspace
    ON ${SCHEMA_NAME}.careers_gallery_items(workspace_id, section, display_order ASC, id ASC);
