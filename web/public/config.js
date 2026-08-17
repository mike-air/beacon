/**
 * Runtime configuration, overwritten by the container at start-up.
 *
 * Vite inlines import.meta.env at BUILD time, so a bundle built with an API
 * base has that base welded into it. One image per environment is then the
 * only option, which means the artefact you tested in staging is not the
 * artefact you ship to production — the thing containers exist to prevent.
 *
 * So the base is read from here instead. In DEV this file is what the Vite
 * server serves, and it stays empty so the build-time value wins — `npm run
 * dev` needs no extra step. In the CONTAINER nginx ignores this copy and
 * aliases /config.js to a file the entrypoint writes from $BEACON_API_BASE,
 * so one image runs in any environment.
 */
window.__BEACON__ = {};
