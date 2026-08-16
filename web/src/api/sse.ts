/**
 * Server-sent events, over fetch rather than EventSource.
 *
 * EventSource is the obvious choice and it cannot be used here: it sends no
 * Authorization header, and Beacon reads the token from that header only. The
 * common workaround is `?access_token=<jwt>`, which writes a live credential
 * into every access log, proxy cache and Referer along the way. Streaming the
 * response body with fetch keeps the token in the header where it belongs.
 *
 * What EventSource gives away, this has to provide:
 *   - reconnect with backoff, and a cap so a dead server is not hammered
 *   - a pause while the tab is hidden — a background tab holding an open
 *     stream is a connection the server is paying for and nobody is reading
 *   - parsing the wire format: `event:`, `data:`, blank-line-terminated, with
 *     `:` comment lines as heartbeats
 *
 * The stream is a HINT, never the source of truth. Beacon drops events for a
 * slow subscriber rather than blocking its publisher, so a client that treats
 * the stream as authoritative will quietly show a stale board. Every event
 * here invalidates a query; the refetch is what is believed.
 */
import { API_BASE } from "./client";
import { getToken } from "./session";

export type SseEvent = { type: string; data: unknown };

type Options = {
  path: string;
  onEvent: (e: SseEvent) => void;
  onStatusChange?: (connected: boolean) => void;
};

const BASE_DELAY_MS = 1_000;
const MAX_DELAY_MS = 30_000;

export function connectSse({ path, onEvent, onStatusChange }: Options): () => void {
  let closed = false;
  let attempt = 0;
  let controller: AbortController | null = null;
  let retryTimer: number | undefined;

  const setConnected = (v: boolean) => onStatusChange?.(v);

  function dispatch(block: string) {
    // A block is one message: lines until a blank line. `:` lines are
    // heartbeats and carry no payload.
    let type = "message";
    const dataLines: string[] = [];
    for (const line of block.split("\n")) {
      if (line.startsWith(":") || line === "") continue;
      if (line.startsWith("event:")) type = line.slice(6).trim();
      else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
    }
    if (dataLines.length === 0) return;
    const raw = dataLines.join("\n");
    try {
      onEvent({ type, data: JSON.parse(raw) });
    } catch {
      onEvent({ type, data: raw });
    }
  }

  async function run() {
    if (closed) return;
    if (document.visibilityState === "hidden") return; // resumed on visibilitychange

    const token = getToken();
    if (!token) return;

    controller = new AbortController();
    try {
      const res = await fetch(API_BASE + path, {
        headers: { Authorization: `Bearer ${token}`, Accept: "text/event-stream" },
        signal: controller.signal,
      });

      if (!res.ok || !res.body) throw new Error(`stream failed: ${res.status}`);

      attempt = 0; // a successful connect resets the backoff
      setConnected(true);

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        // Messages are separated by a blank line. Keep the tail: it may be a
        // half-received message, and parsing half a message loses it.
        let split: number;
        while ((split = buffer.indexOf("\n\n")) !== -1) {
          dispatch(buffer.slice(0, split));
          buffer = buffer.slice(split + 2);
        }
      }
    } catch {
      // Aborts land here too; `closed` distinguishes them from a real failure.
    } finally {
      setConnected(false);
      if (!closed) scheduleRetry();
    }
  }

  function scheduleRetry() {
    const delay = Math.min(BASE_DELAY_MS * 2 ** attempt, MAX_DELAY_MS);
    attempt += 1;
    // Jitter, so a server restart does not bring every client back at once.
    const jittered = delay * (0.7 + Math.random() * 0.6);
    retryTimer = window.setTimeout(run, jittered);
  }

  function onVisibility() {
    if (document.visibilityState === "visible") {
      if (!controller || controller.signal.aborted) {
        attempt = 0;
        void run();
      }
    } else {
      controller?.abort();
    }
  }

  document.addEventListener("visibilitychange", onVisibility);
  void run();

  return () => {
    closed = true;
    document.removeEventListener("visibilitychange", onVisibility);
    if (retryTimer) clearTimeout(retryTimer);
    controller?.abort();
  };
}
