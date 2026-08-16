/**
 * Every call the app makes, in one place.
 *
 * This is a thin naming layer over @beacon/sdk. The SDK already knows the
 * URLs, the shapes and the docs; what it does not know is which calls THIS
 * app makes, or that a page of tasks should be fetched a hundred at a time.
 * Keeping that here means a component never assembles a request, and the
 * whole surface the app depends on is one readable file.
 *
 * Responses are still parsed at the boundary. A generated type is a claim
 * about a server you do not control; a parser is a check. The two are kept in
 * step by the compile-time assertions in parsers.ts.
 */
import { beacon, unwrap } from "@beacon/sdk";
import {
  attachmentSchema,
  commentSchema,
  memberSchema,
  orgSchema,
  orgWithRoleSchema,
  pageParser,
  parser,
  preferencesSchema,
  projectSchema,
  searchResultSchema,
  taskSchema,
  userSchema,
  webhookSchema,
} from "./parsers";
import type { AttachmentEnvelope, Preferences, Project, Role, Task, TaskStatus, User } from "./types";

type Page = { limit?: number; offset?: number };

// ---- auth ------------------------------------------------------------------

export const auth = {
  signup: (body: { email: string; name: string; password: string }): Promise<User> =>
    unwrap(beacon.signup({ body })).then(parser(userSchema, "User")),

  login: (body: { email: string; password: string }) =>
    unwrap(beacon.login({ body })).then((r) => ({
      token: r.token,
      user: parser(userSchema, "User")(r.user),
    })),
};

// ---- me --------------------------------------------------------------------

export const me = {
  get: (): Promise<User> => unwrap(beacon.getMe()).then(parser(userSchema, "User")),

  preferences: (): Promise<Preferences> =>
    unwrap(beacon.getPreferences()).then(parser(preferencesSchema, "Preferences")),

  /**
   * Returns the updated preferences, including what the cascade resolved for
   * this request — so the caller learns what the change actually did without
   * a second round trip.
   */
  setPreferences: (body: { locale?: string; timezone?: string }): Promise<Preferences> =>
    unwrap(beacon.setPreferences({ body })).then(parser(preferencesSchema, "Preferences")),
};

// ---- orgs ------------------------------------------------------------------

export const orgs = {
  list: (page?: Page) =>
    unwrap(beacon.listOrgs({ query: { ...page } })).then(pageParser(orgWithRoleSchema, "Orgs")),

  create: (body: { name: string }) =>
    unwrap(beacon.createOrg({ body })).then(parser(orgSchema, "Org")),

  members: (orgID: string, page?: Page) =>
    unwrap(beacon.listMembers({ path: { orgID }, query: { ...page } })).then(
      pageParser(memberSchema, "Members"),
    ),

  addMember: (orgID: string, body: { email: string; role: Role }) =>
    unwrap(beacon.addMember({ path: { orgID }, body })),

  search: (orgID: string, q: string, page?: Page, opts?: { signal?: AbortSignal }) =>
    unwrap(
      beacon.search({ path: { orgID }, query: { q, ...page }, signal: opts?.signal }),
    ).then(parser(searchResultSchema, "Search")),
};

// ---- projects --------------------------------------------------------------

export const projects = {
  /**
   * `board` only appears on the v2 arm of the new_board_ui experiment. It is
   * returned raw beside the parsed rows so the caller renders the arm the
   * SERVER put this user in — the client never decides.
   */
  list: async (orgID: string, page?: Page) => {
    const raw = await unwrap(beacon.listProjects({ path: { orgID }, query: { ...page } }));
    const parsed = pageParser(projectSchema, "Projects")(raw);
    return { ...parsed, board: raw.board === "v2" ? ("v2" as const) : ("v1" as const) };
  },

  get: (orgID: string, projectID: string): Promise<Project> =>
    unwrap(beacon.getProject({ path: { orgID, projectID } })).then(parser(projectSchema, "Project")),

  create: (orgID: string, body: { name: string }): Promise<Project> =>
    unwrap(beacon.createProject({ path: { orgID }, body })).then(parser(projectSchema, "Project")),

  update: (orgID: string, projectID: string, body: { name: string }): Promise<Project> =>
    unwrap(beacon.updateProject({ path: { orgID, projectID }, body })).then(
      parser(projectSchema, "Project"),
    ),

  remove: (orgID: string, projectID: string) =>
    unwrap(beacon.deleteProject({ path: { orgID, projectID } })),
};

// ---- tasks -----------------------------------------------------------------

export const tasks = {
  list: (orgID: string, projectID: string, page?: Page) =>
    unwrap(
      beacon.listTasks({ path: { orgID, projectID }, query: { limit: 100, ...page } }),
    ).then(pageParser(taskSchema, "Tasks")),

  get: (orgID: string, projectID: string, taskID: string): Promise<Task> =>
    unwrap(beacon.getTask({ path: { orgID, projectID, taskID } })).then(parser(taskSchema, "Task")),

  create: (
    orgID: string,
    projectID: string,
    body: { title: string; status?: TaskStatus; position?: number },
  ): Promise<Task> =>
    unwrap(beacon.createTask({ path: { orgID, projectID }, body })).then(
      parser(taskSchema, "Task"),
    ),

  /** A full replacement of the mutable fields — title and status are both required. */
  update: (
    orgID: string,
    projectID: string,
    taskID: string,
    body: { title: string; status: TaskStatus; position?: number },
  ): Promise<Task> =>
    unwrap(beacon.updateTask({ path: { orgID, projectID, taskID }, body })).then(
      parser(taskSchema, "Task"),
    ),

  remove: (orgID: string, projectID: string, taskID: string) =>
    unwrap(beacon.deleteTask({ path: { orgID, projectID, taskID } })),

  comments: (orgID: string, projectID: string, taskID: string, page?: Page) =>
    unwrap(
      beacon.listComments({ path: { orgID, projectID, taskID }, query: { limit: 100, ...page } }),
    ).then(pageParser(commentSchema, "Comments")),

  comment: (orgID: string, projectID: string, taskID: string, body: { body: string }) =>
    unwrap(beacon.createComment({ path: { orgID, projectID, taskID }, body })).then(
      parser(commentSchema, "Comment"),
    ),

  attachments: (orgID: string, projectID: string, taskID: string) =>
    unwrap(beacon.listAttachments({ path: { orgID, projectID, taskID } })).then(
      pageParser(attachmentSchema, "Attachments"),
    ),

  createAttachment: (
    orgID: string,
    projectID: string,
    taskID: string,
    body: { filename: string; content_type: string; size: number },
  ): Promise<AttachmentEnvelope> =>
    unwrap(beacon.createAttachment({ path: { orgID, projectID, taskID }, body })),

  getAttachment: (orgID: string, projectID: string, taskID: string, attachmentID: string) =>
    unwrap(beacon.getAttachment({ path: { orgID, projectID, taskID, attachmentID } })),
};

// ---- webhooks --------------------------------------------------------------

export const webhooks = {
  list: (orgID: string) =>
    unwrap(beacon.listWebhooks({ path: { orgID } })).then(pageParser(webhookSchema, "Webhooks")),

  register: (orgID: string, body: { url: string; events?: string[] }) =>
    unwrap(beacon.registerWebhook({ path: { orgID }, body })).then(
      parser(webhookSchema, "Webhook"),
    ),

  remove: (orgID: string, webhookID: string) =>
    unwrap(beacon.deleteWebhook({ path: { orgID, webhookID } })),
};

/**
 * The SSE path.
 *
 * Not an SDK method: the stream is not a request and a response, so it is not
 * a generated operation. sse.ts consumes it with fetch, because the token goes
 * in the Authorization header and EventSource cannot set one.
 */
export function eventsPath(orgID: string): string {
  return `/v1/orgs/${orgID}/events`;
}
