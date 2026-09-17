import { useState, useEffect } from "react";

const API = import.meta.env.VITE_API_URL || "http://localhost:8080";

interface Tenant {
  id: string; name: string; upstream_url: string;
  rate_limit_per_second: number; rate_limit_per_minute: number;
  rate_limit_per_day: number; enabled: boolean; created_at: string;
}

const S: Record<string, React.CSSProperties> = {
  app:    { fontFamily: "'JetBrains Mono',monospace", background: "#060610", minHeight: "100vh", color: "#e8e6f0", padding: "2rem" },
  header: { marginBottom: "2rem", borderBottom: "1px solid rgba(255,255,255,0.08)", paddingBottom: "1rem" },
  title:  { fontSize: "1.1rem", fontWeight: 600, margin: 0 },
  sub:    { fontSize: "0.75rem", color: "rgba(232,230,240,0.4)", margin: "0.25rem 0 0" },
  row:    { background: "#0d0d1a", padding: "1rem 1.25rem", display: "grid", gridTemplateColumns: "1fr auto auto", gap: "1rem", alignItems: "center" },
  grid:   { display: "flex", flexDirection: "column", gap: "1px", background: "rgba(255,255,255,0.05)", borderRadius: "10px", overflow: "hidden" },
  err:    { padding: "0.75rem 1rem", background: "rgba(248,113,113,0.1)", border: "1px solid rgba(248,113,113,0.3)", borderRadius: "8px", color: "#f87171", fontSize: "0.8rem", marginBottom: "1rem" },
  empty:  { padding: "2rem", textAlign: "center", color: "rgba(232,230,240,0.3)", fontSize: "0.8rem" },
};

function btn(bg = "rgba(255,255,255,0.08)", fs = "0.78rem"): React.CSSProperties {
  return { background: bg, border: "1px solid rgba(255,255,255,0.08)", borderRadius: "6px", padding: "0.5rem 1rem", color: "#e8e6f0", fontSize: fs, fontFamily: "inherit", cursor: "pointer", whiteSpace: "nowrap" };
}

function inp(): React.CSSProperties {
  return { background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)", borderRadius: "6px", padding: "0.5rem 0.75rem", color: "#e8e6f0", fontSize: "0.8rem", fontFamily: "inherit", outline: "none", width: "100%", boxSizing: "border-box" };
}

export default function App() {
  const [tenants, setTenants]   = useState<Tenant[]>([]);
  const [loading, setLoading]   = useState(true);
  const [error,   setError]     = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const fetchTenants = async () => {
    try {
      const res  = await fetch(`${API}/admin/tenants`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setTenants(data.tenants || []);
    } catch (e: any) { setError(e.message); }
    finally { setLoading(false); }
  };

  useEffect(() => { fetchTenants(); }, []);

  return (
    <div style={S.app}>
      <header style={S.header}>
        <h1 style={S.title}><span style={{ color: "#7c6df0" }}>◈</span> API Gateway Admin</h1>
        <p style={S.sub}>Multi-tenant gateway · {API}</p>
      </header>

      <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "1rem" }}>
        <span style={{ fontSize: "0.8rem", color: "rgba(232,230,240,0.5)" }}>Tenants ({tenants.length})</span>
        <button onClick={() => setCreating(true)} style={btn("#7c6df0")}>+ Nouveau</button>
      </div>

      {error && <div style={S.err}>⚠ {error}</div>}

      {loading ? (
        <p style={{ color: "rgba(232,230,240,0.3)", fontSize: "0.8rem" }}>Chargement…</p>
      ) : (
        <div style={S.grid}>
          {tenants.length === 0
            ? <div style={S.empty}>Aucun tenant — crée le premier</div>
            : tenants.map(t => <TenantRow key={t.id} tenant={t} onRefresh={fetchTenants} />)
          }
        </div>
      )}

      {creating && <CreateModal onClose={() => setCreating(false)} onCreated={() => { setCreating(false); fetchTenants(); }} />}
    </div>
  );
}

function TenantRow({ tenant: t, onRefresh }: { tenant: Tenant; onRefresh: () => void }) {
  const toggle = async () => {
    await fetch(`${API}/admin/tenants/${t.id}`, {
      method: "PUT", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...t, enabled: !t.enabled }),
    });
    onRefresh();
  };

  return (
    <div style={S.row}>
      <div>
        <div style={{ fontSize: "0.88rem", fontWeight: 500 }}>
          {t.name} <span style={{ color: "rgba(232,230,240,0.3)", fontSize: "0.7rem" }}>{t.id.slice(0, 8)}…</span>
        </div>
        <div style={{ fontSize: "0.72rem", color: "rgba(232,230,240,0.4)", marginTop: "0.2rem" }}>
          {t.upstream_url} · {t.rate_limit_per_second}/s · {t.rate_limit_per_minute}/min
        </div>
      </div>
      <span style={{ fontSize: "0.68rem", padding: "2px 8px", borderRadius: "20px",
        background: t.enabled ? "rgba(74,222,128,0.1)" : "rgba(248,113,113,0.1)",
        color: t.enabled ? "#4ade80" : "#f87171",
        border: `1px solid ${t.enabled ? "rgba(74,222,128,0.3)" : "rgba(248,113,113,0.3)"}` }}>
        {t.enabled ? "● actif" : "○ inactif"}
      </span>
      <button onClick={toggle} style={btn()}>{t.enabled ? "Désactiver" : "Activer"}</button>
    </div>
  );
}

function CreateModal({ onClose, onCreated }: { onClose: () => void; onCreated: () => void }) {
  const [form, setForm] = useState({ name: "", upstream_url: "", rate_limit_per_second: 10, rate_limit_per_minute: 100, rate_limit_per_day: 10000 });
  const [saving, setSaving] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault(); setSaving(true);
    try {
      const res = await fetch(`${API}/admin/tenants`, {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...form, enabled: true }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      onCreated();
    } finally { setSaving(false); }
  };

  return (
    <div style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.7)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 100 }} onClick={onClose}>
      <div style={{ background: "#0d0d1a", border: "1px solid rgba(255,255,255,0.08)", borderRadius: "12px", padding: "1.5rem", width: "400px" }} onClick={e => e.stopPropagation()}>
        <h3 style={{ fontSize: "0.9rem", margin: "0 0 1.25rem" }}>Nouveau tenant</h3>
        <form onSubmit={submit} style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
          {[{ label: "Nom", key: "name", type: "text", ph: "Mon service" }, { label: "Upstream URL", key: "upstream_url", type: "url", ph: "http://service:8081" }].map(({ label, key, type, ph }) => (
            <label key={key} style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
              <span style={{ fontSize: "0.72rem", color: "rgba(232,230,240,0.45)" }}>{label}</span>
              <input type={type} value={(form as any)[key]} onChange={e => setForm({ ...form, [key]: e.target.value })} placeholder={ph} required style={inp()} />
            </label>
          ))}
          {[{ label: "Req/s", key: "rate_limit_per_second" }, { label: "Req/min", key: "rate_limit_per_minute" }, { label: "Req/jour", key: "rate_limit_per_day" }].map(({ label, key }) => (
            <label key={key} style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
              <span style={{ fontSize: "0.72rem", color: "rgba(232,230,240,0.45)" }}>{label}</span>
              <input type="number" value={(form as any)[key]} onChange={e => setForm({ ...form, [key]: parseInt(e.target.value) || 0 })} min={0} style={inp()} />
            </label>
          ))}
          <div style={{ display: "flex", gap: "0.5rem", marginTop: "0.5rem" }}>
            <button type="submit" disabled={saving} style={{ ...btn("#7c6df0"), flex: 1 }}>{saving ? "Création…" : "Créer"}</button>
            <button type="button" onClick={onClose} style={btn()}>Annuler</button>
          </div>
        </form>
      </div>
    </div>
  );
}
