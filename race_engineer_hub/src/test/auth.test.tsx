import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, LoginScreen, useAuth } from "@/lib/auth";

function Probe() {
  const { authed } = useAuth();
  if (authed === null) return <p>loading</p>;
  return authed ? <p>dashboard</p> : <LoginScreen />;
}

describe("web authentication", () => {
  afterEach(() => vi.restoreAllMocks());

  it("shows login and enters the dashboard after a successful password", async () => {
    const fetchMock = vi.fn(async (url: string | URL | Request) => {
      const u = String(url);
      if (u.includes("/auth/status")) return new Response(JSON.stringify({ authenticated: false, auth_configured: true, origin_check: true }), { status: 200 });
      if (u.includes("/auth/login")) return new Response(JSON.stringify({ authenticated: true }), { status: 200 });
      return new Response("{}", { status: 200 });
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<AuthProvider><Probe /></AuthProvider>);
    await waitFor(() => expect(screen.getByLabelText("Password")).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "test-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(screen.getByText("dashboard")).toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/auth/login", expect.objectContaining({ method: "POST" }));
  });
});
