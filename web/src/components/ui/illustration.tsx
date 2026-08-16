/**
 * Empty and error states.
 *
 * Every illustration is drawn from the token palette via currentColor and the
 * fill-* utilities, so it re-themes with the app. No PNGs, no assets to load,
 * nothing that looks wrong in dark mode.
 */
import { cn } from "@/lib/cn";

const box = "size-full";

export function EmptyBoard({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 200 140" fill="none" className={cn(box, className)} aria-hidden>
      {[8, 70, 132].map((x, i) => (
        <rect
          key={x}
          x={x}
          y={20}
          width={60}
          height={100}
          rx={8}
          className="fill-well stroke-line"
          strokeWidth={1.5}
          strokeDasharray={i === 1 ? "4 4" : undefined}
        />
      ))}
      <rect x={16} y={32} width={44} height={12} rx={4} className="fill-line-strong" />
      <rect x={16} y={52} width={30} height={8} rx={3} className="fill-line" />
      <rect x={78} y={32} width={44} height={12} rx={4} className="fill-accent-subtle" />
      <circle cx={100} cy={78} r={12} className="fill-volt-subtle" />
      <path d="M94 78 L100 84 L112 70" className="stroke-volt-text" strokeWidth={2.5} fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function EmptyProjects({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 200 140" fill="none" className={cn(box, className)} aria-hidden>
      <rect x={30} y={34} width={140} height={26} rx={7} className="fill-raised stroke-line" strokeWidth={1.5} />
      <rect x={30} y={68} width={140} height={26} rx={7} className="fill-raised stroke-line" strokeWidth={1.5} />
      <rect x={30} y={102} width={140} height={26} rx={7} className="fill-well stroke-line" strokeWidth={1.5} strokeDasharray="4 4" />
      <rect x={42} y={44} width={54} height={7} rx={3.5} className="fill-line-strong" />
      <rect x={42} y={78} width={38} height={7} rx={3.5} className="fill-line-strong" />
      <circle cx={152} cy={47} r={5} className="fill-accent" />
      <circle cx={152} cy={81} r={5} className="fill-success" />
      <path d="M100 108 v14 M93 115 h14" className="stroke-ink-faint" strokeWidth={2} strokeLinecap="round" />
    </svg>
  );
}

export function EmptySearch({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 200 140" fill="none" className={cn(box, className)} aria-hidden>
      <circle cx={88} cy={62} r={34} className="fill-well stroke-line-strong" strokeWidth={2} />
      <path d="M113 87 L146 118" className="stroke-ink-faint" strokeWidth={5} strokeLinecap="round" />
      <path d="M74 62 h28 M74 74 h18" className="stroke-line-strong" strokeWidth={3} strokeLinecap="round" />
    </svg>
  );
}

export function BrokenBeacon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 200 140" fill="none" className={cn(box, className)} aria-hidden>
      <rect x={84} y={30} width={26} height={82} rx={9} className="fill-well stroke-line-strong" strokeWidth={2} />
      <circle cx={97} cy={58} r={8} className="fill-danger-subtle stroke-danger" strokeWidth={2} />
      <path d="M124 44 l26 -10 M124 58 l30 2 M124 72 l24 12" className="stroke-line-strong" strokeWidth={2.5} strokeLinecap="round" strokeDasharray="3 6" />
      <path d="M60 122 h80" className="stroke-line" strokeWidth={2} strokeLinecap="round" />
    </svg>
  );
}
