/**
 * Friendly names for the SDK's generated types, plus the handful of domain
 * facts the client needs that the contract does not carry.
 *
 * Nothing in the app imports from @beacon/sdk/generated directly — it imports
 * these. When the server changes, `make sdk` regenerates the package and every
 * mismatch surfaces here first, in one file, instead of in forty components.
 */
import type {
  Attachment as SdkAttachment,
  AttachmentEnvelope as SdkAttachmentEnvelope,
  Comment as SdkComment,
  FieldError as SdkFieldError,
  Hit as SdkHit,
  Member as SdkMember,
  Membership as SdkMembership,
  Org as SdkOrg,
  OrgWithRole as SdkOrgWithRole,
  PrefsResponse,
  Project as SdkProject,
  SearchResult as SdkSearchResult,
  Task as SdkTask,
  User as SdkUser,
  Webhook as SdkWebhook,
} from "@beacon/sdk";

export type User = SdkUser;
export type Org = SdkOrg;
export type OrgWithRole = SdkOrgWithRole;
export type Member = SdkMember;
export type Membership = SdkMembership;
export type Project = SdkProject;
export type Task = SdkTask;
export type Comment = SdkComment;
export type Attachment = SdkAttachment;
export type AttachmentEnvelope = SdkAttachmentEnvelope;
export type Webhook = SdkWebhook;
export type SearchHit = SdkHit;
export type SearchResult = SdkSearchResult;
export type Preferences = PrefsResponse;
export type FieldError = SdkFieldError;

/**
 * Role and TaskStatus are derived from the generated types rather than
 * declared here. When Beacon adds a fourth status the union widens by itself
 * and every non-exhaustive switch over it becomes a compile error — which is
 * the whole point of generating types instead of writing them.
 */
export type Role = NonNullable<SdkMember["role"]>;
export type TaskStatus = NonNullable<SdkTask["status"]>;

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
