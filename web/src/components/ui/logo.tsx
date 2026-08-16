import { cn } from "@/lib/cn";

/**
 * The mark: a lighthouse tower with a beam. The beam is volt, the tower is
 * accent — the two colors that carry the brand. `live` sweeps the beam, and
 * is driven by the SSE connection, so the logo tells you the truth about
 * whether the app is connected.
 */
export function Logo({
  size = 20,
  live = false,
  className,
}: {
  size?: number;
  live?: boolean;
  className?: string;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      aria-hidden
      className={cn("shrink-0", className)}
    >
      <g className={cn("origin-[7px_16px]", live && "animate-beam")}>
        <path d="M9 16 L29 8.5 L29 23.5 Z" className="fill-volt opacity-35" />
        <path d="M9 16 L24 11.5 L24 20.5 Z" className="fill-volt opacity-75" />
      </g>
      <rect x="4" y="6" width="6" height="20" rx="2.6" className="fill-accent" />
      <circle cx="7" cy="16" r="2" className="fill-page" />
    </svg>
  );
}

export function Wordmark({ live = false }: { live?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2">
      <Logo live={live} />
      <span className="font-display text-[15px] tracking-tight text-ink">BEACON</span>
    </span>
  );
}
