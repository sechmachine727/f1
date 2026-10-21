import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

type AuthValue = {
  authed: boolean | null;
  originCheck: boolean;
  login: (password: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [originCheck, setOriginCheck] = useState(true);

  useEffect(() => {
    fetch("/api/v1/auth/status")
      .then((r) => r.json())
      .then((v) => {
        setAuthed(v.auth_configured === false || v.authenticated === true);
        setOriginCheck(v.origin_check !== false);
      })
      .catch(() => setAuthed(false));
    const required = () => setAuthed(false);
    window.addEventListener("f1tele:auth-required", required);
    return () => window.removeEventListener("f1tele:auth-required", required);
  }, []);

  async function login(password: string) {
    const res = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ message: "Login failed" }));
      throw new Error(body.message ?? "Login failed");
    }
    setAuthed(true);
  }

  async function logout() {
    await fetch("/api/v1/auth/logout", { method: "POST" }).catch(() => null);
    setAuthed(false);
  }

  return <AuthContext.Provider value={{ authed, originCheck, login, logout }}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const v = useContext(AuthContext);
  if (!v) throw new Error("useAuth outside AuthProvider");
  return v;
}

export function LoginScreen() {
  const { login } = useAuth();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await login(password);
    } catch (err) {
      setError(String(err instanceof Error ? err.message : err));
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="flex min-h-screen items-center justify-center bg-[#0b0d11] p-6 text-neutral-100">
      <form onSubmit={submit} className="w-full max-w-sm border border-[#232830] bg-[#11141a] p-6">
        <h1 className="text-lg font-semibold">F1 Telemetry</h1>
        <p className="mt-1 text-sm text-neutral-400">Enter the dashboard password to continue.</p>
        <label htmlFor="web-password" className="mt-5 block text-[11px] uppercase tracking-wider text-neutral-400">Password</label>
        <input
          id="web-password"
          type="password"
          autoFocus
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="mt-1 w-full border border-[#2a3038] bg-[#151920] px-3 py-2 text-sm outline-none focus-visible:border-[#5fd68f]"
        />
        {error ? <p role="alert" className="mt-3 text-[13px] text-[#f25a5c]">{error}</p> : null}
        <button type="submit" disabled={busy || !password} className="mt-4 w-full border border-[#2f5a3d] bg-[#1d3b2c] px-3 py-2 text-sm text-[#8ef0b4] disabled:opacity-40">
          {busy ? "Signing in…" : "Sign in"}
        </button>
        <p className="mt-4 text-[11px] leading-relaxed text-neutral-500">
          The password is stored in the server data directory. Docker prints a generated password once on first start.
        </p>
      </form>
    </main>
  );
}
