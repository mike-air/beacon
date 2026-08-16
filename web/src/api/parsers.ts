/**
 * Runtime parsing, and the compile-time proof that it matches the server.
 *
 * The generated types are a promise about what the server sends. Zod checks
 * that promise at runtime. `AssertExact` checks that the Zod schema and the
 * generated type never drift apart — if the server adds a field and you
 * regenerate, the build fails here until the schema is updated. (ch20)
 *
 * Two policies, deliberately different:
 *   parser()     throws — one bad object means the screen cannot be drawn
 *   pageParser() drops the bad row and keeps the page — one corrupt task must
 *                not blank a board of thirty good ones
 */
import { z } from "zod";
import type {
  Attachment,
  Comment,
  Member,
  Org,
  OrgWithRole,
  Preferences,
  Project,
  Task,
  User,
  Webhook,
} from "./types";

/**
 * Compile error unless A and B are structurally identical.
 *
 * Used as `True<Exact<X, Y>>` at each call site, never wrapped in a generic
 * alias of its own: TypeScript checks a generic alias eagerly, against the
 * unresolved parameters, so `type AssertExact<A,B> = True<Exact<A,B>>` fails
 * on its own declaration before it is ever used.
 *
 * Two details matter. The mismatch arm must be `false`, not `never` — `never`
 * extends everything, so a `never` result would satisfy the constraint and the
 * guard would pass silently. And the check must be a CONSTRAINT (`T extends
 * true`), not a bare alias — an alias that resolves to `false` is a perfectly
 * legal type and raises nothing.
 */
type Exact<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;
type True<T extends true> = T;

const uuid = z.string();
const dateTime = z.string();

export const userSchema = z.object({
  id: uuid,
  email: z.string(),
  name: z.string(),
  created_at: dateTime,
});
export type _UserOk = True<Exact<z.infer<typeof userSchema>, User>>;

export const roleSchema = z.enum(["owner", "admin", "member"]);

export const orgSchema = z.object({
  id: uuid,
  name: z.string(),
  slug: z.string(),
  created_at: dateTime,
});
export type _OrgOk = True<Exact<z.infer<typeof orgSchema>, Org>>;

export const orgWithRoleSchema = orgSchema.extend({ role: roleSchema });
export type _OrgWithRoleOk = True<Exact<z.infer<typeof orgWithRoleSchema>, OrgWithRole>>;

export const memberSchema = z.object({
  user_id: uuid,
  email: z.string(),
  name: z.string(),
  role: roleSchema,
});
export type _MemberOk = True<Exact<z.infer<typeof memberSchema>, Member>>;

export const projectSchema = z.object({
  id: uuid,
  org_id: uuid,
  name: z.string(),
  created_at: dateTime,
  updated_at: dateTime,
});
export type _ProjectOk = True<Exact<z.infer<typeof projectSchema>, Project>>;

export const taskStatusSchema = z.enum(["todo", "in_progress", "done"]);

export const taskSchema = z.object({
  id: uuid,
  org_id: uuid,
  project_id: uuid,
  title: z.string(),
  status: taskStatusSchema,
  position: z.number(),
  created_at: dateTime,
  updated_at: dateTime,
});
export type _TaskOk = True<Exact<z.infer<typeof taskSchema>, Task>>;

export const commentSchema = z.object({
  id: uuid,
  task_id: uuid,
  author_id: uuid,
  body: z.string(),
  created_at: dateTime,
});
export type _CommentOk = True<Exact<z.infer<typeof commentSchema>, Comment>>;

export const attachmentSchema = z.object({
  id: uuid,
  task_id: uuid,
  filename: z.string(),
  content_type: z.string(),
  size_bytes: z.number(),
  storage_key: z.string(),
  created_at: dateTime,
});
export type _AttachmentOk = True<Exact<z.infer<typeof attachmentSchema>, Attachment>>;

export const webhookSchema = z.object({
  id: uuid,
  org_id: uuid,
  url: z.string(),
  secret: z.string().optional(),
  events: z.array(z.string()),
  active: z.boolean(),
  created_at: dateTime,
});
export type _WebhookOk = True<Exact<z.infer<typeof webhookSchema>, Webhook>>;

export const preferencesSchema = z.object({
  locale: z.string(),
  timezone: z.string(),
  resolved_locale: z.string(),
  now_utc: dateTime,
  now_local: z.string(),
  greeting: z.string(),
  example_price: z.string(),
});
export type _PreferencesOk = True<Exact<z.infer<typeof preferencesSchema>, Preferences>>;

export const searchHitSchema = z.object({
  kind: z.string(),
  id: z.string(),
  title: z.string(),
  snippet: z.string(),
  rank: z.number().optional(),
});

export const searchResultSchema = z.object({
  hits: z.array(searchHitSchema),
  source: z.enum(["postgres", "meili"]),
});

/** In dev a shape mismatch is a crash you fix; in prod it is a reported error. */
const isDev = import.meta.env.DEV;

export function parser<T>(schema: z.ZodType<T>, label: string) {
  return (raw: unknown): T => {
    const result = schema.safeParse(raw);
    if (result.success) return result.data;
    const detail = result.error.issues
      .map((i) => `${i.path.join(".") || "(root)"}: ${i.message}`)
      .join("; ");
    const message = `${label} did not match the API contract — ${detail}`;
    if (isDev) throw new Error(message);
    console.error(message, raw);
    throw new Error(`${label} could not be read`);
  };
}

/**
 * A list envelope. Rows that fail are dropped and counted, never thrown —
 * one malformed task must not blank the whole board.
 */
export function pageParser<T>(schema: z.ZodType<T>, label: string) {
  return (raw: unknown): { items: T[]; limit: number; offset: number; dropped: number } => {
    const envelope = z
      .object({ items: z.array(z.unknown()).nullable(), limit: z.number(), offset: z.number() })
      .safeParse(raw);

    if (!envelope.success) {
      const message = `${label} list envelope was not {items, limit, offset}`;
      if (isDev) throw new Error(message);
      console.error(message, raw);
      return { items: [], limit: 0, offset: 0, dropped: 0 };
    }

    const items: T[] = [];
    let dropped = 0;
    for (const row of envelope.data.items ?? []) {
      const parsed = schema.safeParse(row);
      if (parsed.success) items.push(parsed.data);
      else {
        dropped += 1;
        console.error(`${label}: dropped a row that did not parse`, parsed.error.issues, row);
      }
    }
    if (dropped > 0 && isDev) {
      console.warn(`${label}: ${dropped} row(s) dropped. The board is incomplete.`);
    }
    return { items, limit: envelope.data.limit, offset: envelope.data.offset, dropped };
  };
}
