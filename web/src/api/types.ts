/**
 * Friendly names for the generated schema.
 *
 * Nothing in the app imports schema.d.ts directly — it imports these. When the
 * server changes, `npm run generate:api` regenerates schema.d.ts and every
 * mismatch surfaces here first, in one file, instead of in forty components.
 */
import type { components } from "./schema";

type S = components["schemas"];

export type User = S["User"];
export type Org = S["Org"];
export type OrgWithRole = S["OrgWithRole"];
export type Role = S["Role"];
export type Member = S["Member"];
export type Membership = S["Membership"];
export type Project = S["Project"];
export type Task = S["Task"];
export type TaskStatus = S["TaskStatus"];
export type Comment = S["Comment"];
export type Attachment = S["Attachment"];
export type AttachmentEnvelope = S["AttachmentEnvelope"];
export type Webhook = S["Webhook"];
export type SearchHit = S["SearchHit"];
export type SearchResult = S["SearchResult"];
export type Preferences = S["Preferences"];
export type FieldError = S["FieldError"];
export type ProjectList = S["ProjectList"];

/** The three columns of the board, in the order they are shown. */
export const TASK_STATUSES = ["todo", "in_progress", "done"] as const satisfies readonly TaskStatus[];

export const STATUS_LABEL: Record<TaskStatus, string> = {
  todo: "Todo",
  in_progress: "In progress",
  done: "Done",
};

export const ROLES = ["owner", "admin", "member"] as const satisfies readonly Role[];

/** Can this role administer the org? Mirrors requireRole(RoleAdmin) server-side. */
export function isAdmin(role: Role | undefined): boolean {
  return role === "owner" || role === "admin";
}
