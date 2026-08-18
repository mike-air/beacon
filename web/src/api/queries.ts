/**
 * Query keys and hooks.
 *
 * The keys are a hierarchy, so an SSE event can invalidate a whole subtree
 * ("anything about this org's tasks") without naming every query.
 */
import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { BeaconError } from "@beacon/sdk";
import { me, orgs, projects, tasks, webhooks } from "./endpoints";
import type { Role, TaskStatus } from "./types";

export const keys = {
  me: ["me"] as const,
  preferences: ["me", "preferences"] as const,
  orgs: ["orgs"] as const,
  org: (orgID: string) => ["orgs", orgID] as const,
  members: (orgID: string) => ["orgs", orgID, "members"] as const,
  webhooks: (orgID: string) => ["orgs", orgID, "webhooks"] as const,
  projects: (orgID: string) => ["orgs", orgID, "projects"] as const,
  project: (orgID: string, projectID: string) =>
    ["orgs", orgID, "projects", projectID] as const,
  tasks: (orgID: string, projectID: string) =>
    ["orgs", orgID, "projects", projectID, "tasks"] as const,
  task: (orgID: string, projectID: string, taskID: string) =>
    ["orgs", orgID, "projects", projectID, "tasks", taskID] as const,
  comments: (orgID: string, projectID: string, taskID: string) =>
    ["orgs", orgID, "projects", projectID, "tasks", taskID, "comments"] as const,
  attachments: (orgID: string, projectID: string, taskID: string) =>
    ["orgs", orgID, "projects", projectID, "tasks", taskID, "attachments"] as const,
  search: (orgID: string, q: string) => ["orgs", orgID, "search", q] as const,
};

/**
 * Retry policy. A 4xx means the request was wrong; sending it again three
 * times is three more wrong requests. Only 429 and 5xx are worth another go,
 * and the client layer has already honoured Retry-After for those.
 */
export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof BeaconError && !error.isRetryable) return false;
  return failureCount < 2;
}

// ---- reads -----------------------------------------------------------------

export function useMe() {
  return useQuery({ queryKey: keys.me, queryFn: () => me.get(), retry: shouldRetry });
}

export function usePreferences() {
  return useQuery({
    queryKey: keys.preferences,
    queryFn: () => me.preferences(),
    retry: shouldRetry,
  });
}

export function useOrgs() {
  return useQuery({ queryKey: keys.orgs, queryFn: () => orgs.list(), retry: shouldRetry });
}

export function useMembers(orgID: string | undefined) {
  return useQuery({
    queryKey: keys.members(orgID ?? ""),
    queryFn: () => orgs.members(orgID!),
    enabled: !!orgID,
    retry: shouldRetry,
  });
}

export function useWebhooks(orgID: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: keys.webhooks(orgID ?? ""),
    queryFn: () => webhooks.list(orgID!),
    enabled: !!orgID && enabled,
    retry: shouldRetry,
  });
}

export function useProjects(orgID: string | undefined) {
  return useQuery({
    queryKey: keys.projects(orgID ?? ""),
    queryFn: () => projects.list(orgID!),
    enabled: !!orgID,
    retry: shouldRetry,
  });
}

export function useTasks(orgID: string | undefined, projectID: string | undefined) {
  return useQuery({
    queryKey: keys.tasks(orgID ?? "", projectID ?? ""),
    queryFn: () => tasks.list(orgID!, projectID!),
    enabled: !!orgID && !!projectID,
    retry: shouldRetry,
  });
}

export function useComments(orgID: string, projectID: string, taskID: string | undefined) {
  return useQuery({
    queryKey: keys.comments(orgID, projectID, taskID ?? ""),
    queryFn: () => tasks.comments(orgID, projectID, taskID!),
    enabled: !!taskID,
    retry: shouldRetry,
  });
}

export function useAttachments(orgID: string, projectID: string, taskID: string | undefined) {
  return useQuery({
    queryKey: keys.attachments(orgID, projectID, taskID ?? ""),
    queryFn: () => tasks.attachments(orgID, projectID, taskID!),
    enabled: !!taskID,
    retry: shouldRetry,
  });
}

// ---- writes ----------------------------------------------------------------

export function useCreateOrg() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => orgs.create({ name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.orgs }),
  });
}

export function useAddMember(orgID: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { email: string; role: Role }) => orgs.addMember(orgID, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.members(orgID) }),
  });
}

export function useCreateProject(orgID: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => projects.create(orgID, { name }),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.projects(orgID) }),
  });
}

export function useCreateTask(orgID: string, projectID: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { title: string; status: TaskStatus; position: number }) =>
      tasks.create(orgID, projectID, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.tasks(orgID, projectID) }),
  });
}

export function useImportTasks(orgID: string, projectID: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (csv: string) => tasks.importCSV(orgID, projectID, csv),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.tasks(orgID, projectID) }),
  });
}

/** Everything about one org, after an event that could have changed anything. */
export function invalidateOrg(qc: QueryClient, orgID: string) {
  void qc.invalidateQueries({ queryKey: keys.org(orgID) });
}
