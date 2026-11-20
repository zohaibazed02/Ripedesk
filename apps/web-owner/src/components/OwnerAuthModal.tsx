"use client";

import React, { useState } from "react";

export const OWNER_TOKEN_STORAGE_KEY = "ripedesk_owner_token";

interface OwnerAuthModalProps {
  open: boolean;
  cloudApiUrl: string;
  onClose: () => void;
  onAuthenticated: (token: string) => void;
}

export function OwnerAuthModal({
  open,
  cloudApiUrl,
  onClose,
  onAuthenticated,
}: OwnerAuthModalProps) {
  const [mode, setMode] = useState<"login" | "paste">("login");
  const [tenantSlug, setTenantSlug] = useState("artisan-roast-bistro");
  const [email, setEmail] = useState("owner@artisanroast.pk");
  const [password, setPassword] = useState("");
  const [pastedToken, setPastedToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (mode === "paste") {
      const clean = pastedToken.trim().replace(/^Bearer\s+/i, "");
      if (!clean) {
        setError("Please paste a valid Bearer JWT token.");
        return;
      }
      localStorage.setItem(OWNER_TOKEN_STORAGE_KEY, clean);
      onAuthenticated(clean);
      onClose();
      return;
    }

    setLoading(true);
    try {
      const res = await fetch(`${cloudApiUrl}/api/v1/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tenant_slug: tenantSlug.trim(),
          email: email.trim(),
          password,
        }),
      });
      const json = (await res.json()) as {
        data?: { access_token?: string };
        error?: { message?: string };
      };
      if (!res.ok || !json.data?.access_token) {
        throw new Error(json.error?.message || `Login failed (HTTP ${res.status})`);
      }
      localStorage.setItem(OWNER_TOKEN_STORAGE_KEY, json.data.access_token);
      onAuthenticated(json.data.access_token);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to sign in");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl border border-zinc-800 bg-zinc-950 p-6 text-zinc-100 shadow-2xl space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-base font-bold">Owner Sign In</h2>
            <p className="text-xs text-zinc-400">
              Authenticate against {cloudApiUrl}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-zinc-800 px-2.5 py-1 text-xs text-zinc-400 hover:text-white"
          >
            Close
          </button>
        </div>

        <div className="flex gap-1.5 rounded-lg bg-zinc-900 p-1 text-xs">
          <button
            type="button"
            onClick={() => setMode("login")}
            className={`flex-1 rounded-md py-1.5 font-semibold ${
              mode === "login" ? "bg-emerald-600 text-white" : "text-zinc-400"
            }`}
          >
            Email &amp; Password
          </button>
          <button
            type="button"
            onClick={() => setMode("paste")}
            className={`flex-1 rounded-md py-1.5 font-semibold ${
              mode === "paste" ? "bg-emerald-600 text-white" : "text-zinc-400"
            }`}
          >
            Paste Bearer Token
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-3">
          {mode === "login" ? (
            <>
              <div>
                <label className="block text-xs text-zinc-400 mb-1">Tenant Slug</label>
                <input
                  type="text"
                  value={tenantSlug}
                  onChange={(e) => setTenantSlug(e.target.value)}
                  required
                  className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-white focus:border-emerald-500 focus:outline-none"
                />
              </div>
              <div>
                <label className="block text-xs text-zinc-400 mb-1">Owner Email</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-white focus:border-emerald-500 focus:outline-none"
                />
              </div>
              <div>
                <label className="block text-xs text-zinc-400 mb-1">Password</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-white focus:border-emerald-500 focus:outline-none"
                />
              </div>
            </>
          ) : (
            <div>
              <label className="block text-xs text-zinc-400 mb-1">Bearer JWT Token</label>
              <textarea
                rows={4}
                value={pastedToken}
                onChange={(e) => setPastedToken(e.target.value)}
                placeholder="eyJhbGciOiJIUzI1NiIs..."
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 font-mono text-xs text-white focus:border-emerald-500 focus:outline-none"
              />
            </div>
          )}

          {error && (
            <div className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-300">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-emerald-600 py-2.5 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
          >
            {loading ? "Signing in..." : mode === "login" ? "Sign In" : "Save Token & Connect"}
          </button>
        </form>
      </div>
    </div>
  );
}
