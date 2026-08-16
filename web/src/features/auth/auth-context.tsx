import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { auth } from "@/api/endpoints";
import { clearToken, isAuthenticated, setToken, subscribeSession } from "@/api/session";

type AuthValue = {
  signedIn: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signUp: (email: string, name: string, password: string) => Promise<void>;
  /** Clears the session and leaves the app with a full page load. */
  signOut: () => void;
};

const Ctx = createContext<AuthValue | null>(null);

export function useAuth(): AuthValue {
  const v = useContext(Ctx);
  if (!v) throw new Error("useAuth must be used inside <AuthProvider>");
  return v;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  const [signedIn, setSignedIn] = useState(isAuthenticated);

  // The client calls expireSession() on any 401 — including from another tab.
  useEffect(() => subscribeSession(() => setSignedIn(isAuthenticated())), []);

  const signIn = useCallback(
    async (email: string, password: string) => {
      const { token } = await auth.login({ email, password });
      setToken(token);
      setSignedIn(true);
    },
    [],
  );

  const signUp = useCallback(
    async (email: string, name: string, password: string) => {
      // Signup returns the user, not a token. Logging in is a second call, and
      // pretending otherwise would leave the user on a dead screen.
      await auth.signup({ email, name, password });
      await signIn(email, password);
    },
    [signIn],
  );

  const signOut = useCallback(() => {
    clearToken();
    setSignedIn(false);
    // Another account's data must not survive a sign-out in the cache.
    qc.clear();

    // Then LEAVE, with a full document load rather than a client-side route.
    //
    // Leaving is not optional. The route guard is a `beforeLoad`, which
    // TanStack Router runs on navigation and at no other time — so clearing
    // the token while sitting on /settings never re-runs it. The first version
    // of this app simply did not navigate, and the user stayed on the app
    // shell with no token, watching every query 401 in turn.
    //
    // The second version called useNavigate() here, which did nothing at all:
    // AuthProvider wraps RouterProvider in main.tsx, so this component sits
    // ABOVE the router and has no navigation context to use. That compiles
    // perfectly, because the failure is a runtime context lookup.
    //
    // A full load is the honest fix and arguably the better one. Sign-out is
    // precisely the moment you want every in-memory trace of the last account
    // gone — not just the query cache, but any component state, any closure
    // still holding a name or an id. A fresh document guarantees that in a way
    // a client-side transition cannot.
    //
    // Both bugs were found by an e2e test. Neither was visible to the compiler.
    window.location.assign("/sign-in");
  }, [qc]);

  const value = useMemo(
    () => ({ signedIn, signIn, signUp, signOut }),
    [signedIn, signIn, signUp, signOut],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
