import { Wordmark } from "@/components/ui/logo";
import heroJpg from "@/assets/sign-in-hero.jpg";
import heroWebp from "@/assets/sign-in-hero.webp";

/**
 * The signed-out frame. One column on small screens, two from `lg` up — there
 * is exactly one thing to do on these screens and nowhere else to go, so the
 * second column carries no controls, only the image.
 *
 * The photograph is Eierland Lighthouse, Texel (Evgeni Tcherkasski, Unsplash
 * License). It is the one raster asset in the app: everywhere else, art is a
 * token-driven SVG that re-themes (see components/ui/illustration.tsx). A
 * photograph cannot re-theme, so it is confined to the panel and the panel is
 * dark in both themes — the image supplies its own ground rather than
 * borrowing the page's.
 *
 * Phones must not pay for it. `hidden lg:block` is not enough on its own —
 * a browser will happily download an image inside a `display: none` subtree,
 * which was measured here, not assumed. The `media` attribute on each <source>
 * is what actually gates the request: below `lg` no source matches, the <img>
 * falls back to an inline transparent pixel, and nothing goes over the wire.
 */

/** 1x1 transparent GIF. The fallback that costs no request. */
const BLANK =
  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7";
export function AuthLayout({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}) {
  return (
    <div className="grid min-h-screen lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
      <div className="flex min-h-screen flex-col bg-sunken lg:min-h-0">
        <header className="px-6 py-5">
          {/* Not `live`: the beam reports the SSE connection, and there is no
              stream before sign-in. A beam that always sweeps is decoration
              pretending to be a status light. */}
          <Wordmark />
        </header>
        <main className="flex flex-1 items-start justify-center px-6 pb-20 pt-6 sm:pt-14 lg:items-center lg:pb-6 lg:pt-6">
          <div className="w-full max-w-sm">
            <h1 className="font-display text-2xl tracking-tight text-ink">{title}</h1>
            {subtitle && <p className="mt-1.5 text-ui text-ink-muted">{subtitle}</p>}
            <div className="mt-6 rounded-(--radius-card) border border-line bg-raised p-5">
              {children}
            </div>
            {footer && <div className="mt-4 text-center text-ui text-ink-muted">{footer}</div>}
          </div>
        </main>
      </div>

      <aside className="relative hidden overflow-hidden bg-[#0b1220] lg:block">
        <picture>
          <source media="(min-width: 1024px)" type="image/webp" srcSet={heroWebp} />
          <source media="(min-width: 1024px)" type="image/jpeg" srcSet={heroJpg} />
          <img
            src={BLANK}
            alt=""
            aria-hidden
            className="size-full object-cover"
            decoding="async"
          />
        </picture>
        {/* Scrim: the photograph's own sky is bright enough at the horizon to
            fight white text, so the caption sits on a gradient rather than on
            the image itself. */}
        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/85 to-transparent p-10 pt-24">
          <p className="max-w-sm font-display text-xl leading-snug text-white">
            One place your team can see the work from.
          </p>
          <p className="mt-2 text-ui text-white/60">
            Eierland Lighthouse, Texel &middot; Evgeni Tcherkasski
          </p>
        </div>
      </aside>
    </div>
  );
}
