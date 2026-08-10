-- Chapter 31 — feature flags, and Chapter 32 — experiments. Two features, four
-- tables, one shared idea: the decision about who sees what lives in data, not
-- in a deploy.
--
-- [verbatim ch31 + ch32] with the file numbered for this repo's phases and the
-- organizations/users foreign keys pointed at the tables that exist here.

-- Ch 31 — the registry. One row per flag, and its default answer.
CREATE TABLE feature_flags (
    name           text         PRIMARY KEY,
    description    text         NOT NULL DEFAULT '',
    default_value  boolean      NOT NULL DEFAULT false,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    updated_at     timestamptz  NOT NULL DEFAULT now()
);

-- Sparse by design: a row exists only where somebody differs from the default.
-- A thousand orgs on the default cost nothing.
CREATE TABLE feature_flag_overrides (
    flag_name      text         NOT NULL REFERENCES feature_flags(name) ON DELETE CASCADE,
    org_id         uuid         REFERENCES organizations(id) ON DELETE CASCADE,
    user_id        uuid         REFERENCES users(id) ON DELETE CASCADE,
    value          boolean      NOT NULL,
    created_at     timestamptz  NOT NULL DEFAULT now(),
    -- exactly one of org_id / user_id must be set; never both, never neither
    CHECK ((org_id IS NULL) <> (user_id IS NULL))
);

CREATE UNIQUE INDEX feature_flag_overrides_org_uq
    ON feature_flag_overrides (flag_name, org_id) WHERE org_id IS NOT NULL;
CREATE UNIQUE INDEX feature_flag_overrides_user_uq
    ON feature_flag_overrides (flag_name, user_id) WHERE user_id IS NOT NULL;


-- Ch 32 — experiments. Variants live as JSON because their shape is a
-- configuration detail, not a schema the application queries into.
CREATE TABLE experiments (
    key                 text         PRIMARY KEY,
    description         text         NOT NULL DEFAULT '',
    status              text         NOT NULL DEFAULT 'draft',
                        -- 'draft' | 'running' | 'stopped'
    variants            jsonb        NOT NULL,
                        -- [{"name":"control","weight":50},{"name":"treatment","weight":50}]
    started_at          timestamptz,
    stopped_at          timestamptz,
    created_at          timestamptz  NOT NULL DEFAULT now()
);

-- The audit trail, NOT the hot path. The hash answers "which variant" at
-- runtime; this table answers "what did Alice see on the 17th of May", which is
-- the question you cannot reconstruct later if you didn't write it down.
CREATE TABLE experiment_assignments (
    experiment_key      text         NOT NULL REFERENCES experiments(key) ON DELETE CASCADE,
    user_id             uuid         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    variant             text         NOT NULL,
    assigned_at         timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (experiment_key, user_id)
);


-- Seed the flag Chapters 31 and 32 both use as their worked example, plus the
-- experiment that splits the users the flag lets through. Both start OFF and
-- 'draft': a flag that ships switched on is just a deploy with extra steps.
INSERT INTO feature_flags (name, description, default_value) VALUES
    ('new_board_ui',
     'Ch 31/32 worked example. Owner: platform. Expiry: remove once the v2 board is the only board.',
     false)
ON CONFLICT (name) DO NOTHING;

INSERT INTO experiments (key, description, status, variants) VALUES
    ('new_board_ui',
     'Ch 32 worked example: does the v2 board change how projects are listed?',
     'draft',
     '[{"name":"control","weight":50},{"name":"treatment","weight":50}]'::jsonb)
ON CONFLICT (key) DO NOTHING;
