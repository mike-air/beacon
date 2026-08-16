import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { auth } from "@/api/endpoints";
import { clearToken, isAuthenticated, setToken, subscribeSession } from "@/api/session";

type AuthValue = {
  signedIn: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signUp: (email: string, name: string, password: string) => Promise<void>;
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
  }, [qc]);

  const value = useMemo(
    () => ({ signedIn, signIn, signUp, signOut }),
    [signedIn, signIn, signUp, signOut],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
