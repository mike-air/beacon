/**
 * Every URL the app knows, in one place, and every response parsed on the way
 * out. A component never builds a path and never sees an unvalidated object.
 */
import { api, type RequestOptions } from "./client";
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
import type {
  AttachmentEnvelope,
  Org,
  Preferences,
  Project,
  Role,
  Task,
  TaskStatus,
  User,
} from "./types";

const org = (orgID: string) => `/v1/orgs/${orgID}`;
const project = (orgID: string, projectID: string) => `${org(orgID)}/projects/${projectID}`;
const task = (orgID: string, projectID: string, taskID: string) =>
  `${project(orgID, projectID)}/tasks/${taskID}`;

type Page = { limit?: number; offset?: number };

// ---- auth ------------------------------------------------------------------

export const auth = {
  signup: (body: { email: string; name: string; password: string }) =>
    api.post<unknown>("/v1/auth/signup", body).then(parser(userSchema, "User")),

  login: (body: { email: string; password: string }) =>
    api.post<{ token: string; user: unknown }>("/v1/auth/login", body).then((r) => ({
      token: r.token,
      user: parser(userSchema, "User")(r.user),
    })),
};

// ---- me --------------------------------------------------------------------

export const me = {
  get: (opts?: RequestOptions): Promise<User> =>
    api.get<unknown>("/v1/me", opts).then(parser(userSchema, "User")),

  preferences: (): Promise<Preferences> =>
    api.get<unknown>("/v1/me/preferences").then(parser(preferencesSchema, "Preferences")),

  setPreferences: (body: { locale?: string; timezone?: string }): Promise<Preferences> =>
    api.patch<unknown>("/v1/me/preferences", body).then(parser(preferencesSchema, "Preferences")),
};

// ---- orgs ------------------------------------------------------------------

export const orgs = {
  list: (page?: Page) =>
    api.get<unknown>("/v1/orgs", { query: { ...page } }).then(pageParser(orgWithRoleSchema, "Orgs")),

  create: (body: { name: string }, opts?: RequestOptions): Promise<Org> =>
    api.post<unknown>("/v1/orgs", body, opts).then(parser(orgSchema, "Org")),

  members: (orgID: string, page?: Page) =>
    api
      .get<unknown>(`${org(orgID)}/members`, { query: { ...page } })
      .then(pageParser(memberSchema, "Members")),

  addMember: (orgID: string, body: { email: string; role: Role }, opts?: RequestOptions) =>
    api.post<unknown>(`${org(orgID)}/members`, body, opts),

  search: (orgID: string, q: string, page?: Page, opts?: RequestOptions) =>
    api
      .get<unknown>(`${org(orgID)}/search`, { ...opts, query: { q, ...page } })
      .then(parser(searchResultSchema, "Search")),
};

// ---- projects --------------------------------------------------------------

export const projects = {
  /**
   * The `board` marker only appears on the v2 branch of the `new_board_ui`
   * experiment. It is returned raw alongside the parsed rows so the caller can
   * render the arm the SERVER put this user in — the client never decides.
   */
  list: async (orgID: string, page?: Page) => {
    const raw = await api.get<{ board?: string }>(`${org(orgID)}/projects`, { query: { ...page } });
    const parsed = pageParser(projectSchema, "Projects")(raw);
    return { ...parsed, board: raw.board === "v2" ? ("v2" as const) : ("v1" as const) };
  },

  get: (orgID: string, projectID: string): Promise<Project> =>
    api.get<unknown>(project(orgID, projectID)).then(parser(projectSchema, "Project")),

  create: (orgID: string, body: { name: string }, opts?: RequestOptions): Promise<Project> =>
    api.post<unknown>(`${org(orgID)}/projects`, body, opts).then(parser(projectSchema, "Project")),

  update: (orgID: string, projectID: string, body: { name: string }): Promise<Project> =>
    api.patch<unknown>(project(orgID, projectID), body).then(parser(projectSchema, "Project")),

  remove: (orgID: string, projectID: string) => api.del<void>(project(orgID, projectID)),
};

// ---- tasks -----------------------------------------------------------------

export const tasks = {
  list: (orgID: string, projectID: string, page?: Page) =>
    api
      .get<unknown>(`${project(orgID, projectID)}/tasks`, { query: { limit: 100, ...page } })
      .then(pageParser(taskSchema, "Tasks")),

  get: (orgID: string, projectID: string, taskID: string): Promise<Task> =>
    api.get<unknown>(task(orgID, projectID, taskID)).then(parser(taskSchema, "Task")),

  create: (
    orgID: string,
    projectID: string,
    body: { title: string; status?: TaskStatus; position?: number },
    opts?: RequestOptions,
  ): Promise<Task> =>
    api
      .post<unknown>(`${project(orgID, projectID)}/tasks`, body, opts)
      .then(parser(taskSchema, "Task")),

  /** PATCH is a full replacement of the mutable fields — both are required. */
  update: (
    orgID: string,
    projectID: string,
    taskID: string,
    body: { title: string; status: TaskStatus; position?: number },
    opts?: RequestOptions,
  ): Promise<Task> =>
    api.patch<unknown>(task(orgID, projectID, taskID), body, opts).then(parser(taskSchema, "Task")),

  remove: (orgID: string, projectID: string, taskID: string) =>
    api.del<void>(task(orgID, projectID, taskID)),

  comments: (orgID: string, projectID: string, taskID: string, page?: Page) =>
    api
      .get<unknown>(`${task(orgID, projectID, taskID)}/comments`, { query: { limit: 100, ...page } })
      .then(pageParser(commentSchema, "Comments")),

  comment: (
    orgID: string,
    projectID: string,
    taskID: string,
    body: { body: string },
    opts?: RequestOptions,
  ) =>
    api
      .post<unknown>(`${task(orgID, projectID, taskID)}/comments`, body, opts)
      .then(parser(commentSchema, "Comment")),

  attachments: (orgID: string, projectID: string, taskID: string) =>
    api
      .get<unknown>(`${task(orgID, projectID, taskID)}/attachments`)
      .then(pageParser(attachmentSchema, "Attachments")),

  createAttachment: (
    orgID: string,
    projectID: string,
    taskID: string,
    body: { filename: string; content_type: string; size: number },
  ): Promise<AttachmentEnvelope> =>
    api.post<AttachmentEnvelope>(`${task(orgID, projectID, taskID)}/attachments`, body),

  getAttachment: (orgID: string, projectID: string, taskID: string, attachmentID: string) =>
    api.get<AttachmentEnvelope>(
      `${task(orgID, projectID, taskID)}/attachments/${attachmentID}`,
    ),
};

// ---- webhooks --------------------------------------------------------------

export const webhooks = {
  list: (orgID: string) =>
    api.get<unknown>(`${org(orgID)}/webhooks`).then(pageParser(webhookSchema, "Webhooks")),

  register: (orgID: string, body: { url: string; events?: string[] }, opts?: RequestOptions) =>
    api.post<unknown>(`${org(orgID)}/webhooks`, body, opts).then(parser(webhookSchema, "Webhook")),

  remove: (orgID: string, webhookID: string) =>
    api.del<void>(`${org(orgID)}/webhooks/${webhookID}`),
};

/**
 * The SSE endpoint.
 *
 * Beacon's requireAuth reads the token from the Authorization header and
 * nowhere else, and the native EventSource cannot set headers. The usual
 * workaround — `?access_token=` — puts a credential in a URL, where it lands
 * in access logs and proxy caches. So this app streams with fetch instead;
 * see sse.ts.
 */
export function eventsPath(orgID: string): string {
  return `${org(orgID)}/events`;
}
