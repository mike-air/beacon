import {
  createRootRoute,
  createRoute,
  createRouter,
  lazyRouteComponent,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { isAuthenticated } from "@/api/session";
import { SignIn } from "@/features/auth/sign-in";
import { OrgGate } from "@/features/org/org-gate";
import { NotFound } from "@/routes/not-found";

const rootRoute = createRootRoute({ component: Outlet, notFoundComponent: NotFound });

/** Guard: a signed-out user never reaches an app screen. */
function requireAuth({ location }: { location: { href: string } }) {
  if (!isAuthenticated()) {
    throw redirect({ to: "/sign-in", search: { next: location.href } });
  }
}

const signIn = createRoute({
  getParentRoute: () => rootRoute,
  path: "/sign-in",
  component: SignIn,
  // `next` is optional; typing it as `string | undefined` on an object that
  // always has the key keeps <Link to="/sign-in"> valid without a search prop.
  validateSearch: (s: Record<string, unknown>): { next?: string } =>
    typeof s["next"] === "string" ? { next: s["next"] } : {},
});

const signUp = createRoute({
  getParentRoute: () => rootRoute,
  path: "/sign-up",
  component: lazyRouteComponent(() => import("@/features/auth/sign-up"), "SignUp"),
});

const welcome = createRoute({
  getParentRoute: () => rootRoute,
  path: "/welcome",
  beforeLoad: requireAuth,
  component: lazyRouteComponent(() => import("@/features/onboarding/onboarding"), "Onboarding"),
});

/** Everything inside the shell hangs off one resolved org. */
const app = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  beforeLoad: requireAuth,
  component: OrgGate,
});

const projectsIndex = createRoute({
  getParentRoute: () => app,
  path: "/",
  component: lazyRouteComponent(() => import("@/features/projects/projects-index"), "ProjectsIndex"),
});

const board = createRoute({
  getParentRoute: () => app,
  path: "/projects/$projectID",
  component: lazyRouteComponent(() => import("@/features/board/board-page"), "BoardPage"),
  // The open task is in the URL, so a card is a link somebody can send.
  validateSearch: (s: Record<string, unknown>): { task?: string } =>
    typeof s["task"] === "string" ? { task: s["task"] } : {},
});

const members = createRoute({
  getParentRoute: () => app,
  path: "/members",
  component: lazyRouteComponent(() => import("@/features/members/members-page"), "MembersPage"),
});

const settings = createRoute({
  getParentRoute: () => app,
  path: "/settings",
  component: lazyRouteComponent(() => import("@/features/settings/settings-page"), "SettingsPage"),
});

const styleguide = createRoute({
  getParentRoute: () => rootRoute,
  path: "/styleguide",
  component: lazyRouteComponent(() => import("@/routes/styleguide"), "Styleguide"),
});

const routeTree = rootRoute.addChildren([
  signIn,
  signUp,
  welcome,
  styleguide,
  app.addChildren([projectsIndex, board, members, settings]),
]);

export const router = createRouter({ routeTree, defaultPreload: "intent" });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
