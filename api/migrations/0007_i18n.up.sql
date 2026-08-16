-- Chapter 33 — internationalisation. Three columns, and each one exists because
-- the alternative is a bug you cannot fix later.
--
-- locale/timezone are per USER, not per org, because two people on the same
-- team can sit in different countries. default_locale is per ORG so a new
-- member starts somewhere sensible instead of in English.
--
-- The timezone is an IANA name ("Europe/Berlin"), never an offset. An offset
-- is a fact about one instant; a zone is a rule that survives the clocks
-- changing. Store the rule.
--
-- [ch33's schema, which the chapter names in prose (users.locale,
-- users.timezone, the org default) rather than printing as SQL.]

ALTER TABLE users
    ADD COLUMN locale   text NOT NULL DEFAULT '',
    ADD COLUMN timezone text NOT NULL DEFAULT 'UTC';

ALTER TABLE organizations
    ADD COLUMN default_locale text NOT NULL DEFAULT '';
