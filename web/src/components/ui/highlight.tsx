/**
 * Search hits arrive with `<mark>` around the matched terms.
 *
 * Two wrong ways to handle that. Rendering the string as-is shows the user
 * literal `<mark>` tags. Rendering it with dangerouslySetInnerHTML gives a
 * search index — which holds text other people typed — a direct route to
 * executing script in this app.
 *
 * So the markers are parsed, and only the markers: the text between them is
 * put into React as text, which React escapes. `<script>` in a task title
 * comes out as the characters `<script>`, because that is what it is.
 */
const MARK = /<mark>(.*?)<\/mark>/gs;

export function Highlight({ text }: { text: string }) {
  const parts: React.ReactNode[] = [];
  let last = 0;
  let key = 0;

  for (const match of text.matchAll(MARK)) {
    const at = match.index;
    if (at > last) parts.push(text.slice(last, at));
    parts.push(
      <mark key={key++} className="rounded-sm bg-volt-subtle px-0.5 text-volt-text">
        {match[1]}
      </mark>,
    );
    last = at + match[0].length;
  }
  if (last < text.length) parts.push(text.slice(last));

  return <>{parts}</>;
}
