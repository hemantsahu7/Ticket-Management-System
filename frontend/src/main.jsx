import { useEffect, useState } from "react";
import { BrowserRouter, Link, Navigate, Route, Routes, useNavigate } from "react-router-dom";
import { createRoot } from "react-dom/client";
import "./styles.css";

const API = import.meta.env.VITE_API_URL;

if (!API) {
  throw new Error("VITE_API_URL must be set in frontend/.env");
}

async function api(path, options = {}) {
  const token = localStorage.getItem("token");
  const response = await fetch(API + path, { ...options, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers } });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || "Request failed");
  return data;
}

function App() {
  const [token, setToken] = useState(localStorage.getItem("token"));
  const signOut = () => { localStorage.removeItem("token"); setToken(null); };
  return <Routes>
    <Route path="/login" element={<LoginPage token={token} onLogin={setToken} />} />
    <Route path="/register" element={<RegisterPage token={token} />} />
    <Route path="/" element={<ProtectedRoute token={token}><HomePage onSignOut={signOut} /></ProtectedRoute>} />
    <Route path="*" element={<Navigate to={token ? "/" : "/login"} replace />} />
  </Routes>;
}

function ProtectedRoute({ token, children }) { return token ? children : <Navigate to="/login" replace />; }
function AuthPage({ title, children, footer }) { return <main className="auth-page"><section className="card"><h1>Ticket System</h1><h2>{title}</h2>{children}<p className="auth-footer">{footer}</p></section></main>; }
function Message({ text }) { return text ? <p className="message">{text}</p> : null; }

function LoginPage({ token, onLogin }) {
  const navigate = useNavigate(); const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [message, setMessage] = useState("");
  if (token) return <Navigate to="/" replace />;
  async function submit(event) {
    event.preventDefault(); setMessage("");
    try { const data = await api("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }); localStorage.setItem("token", data.token); onLogin(data.token); navigate("/"); }
    catch (error) { setMessage(error.message); }
  }
  return <AuthPage title="Sign in" footer={<>New here? <Link to="/register">Create an account</Link></>}><form onSubmit={submit}><input type="email" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} required /><input type="password" placeholder="Password" minLength="8" value={password} onChange={e => setPassword(e.target.value)} required /><button>Sign in</button></form><Message text={message} /></AuthPage>;
}

function RegisterPage({ token }) {
  const navigate = useNavigate(); const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [message, setMessage] = useState("");
  if (token) return <Navigate to="/" replace />;
  async function submit(event) {
    event.preventDefault(); setMessage("");
    try { await api("/auth/register", { method: "POST", body: JSON.stringify({ email, password }) }); navigate("/login"); }
    catch (error) { setMessage(error.message); }
  }
  return <AuthPage title="Create account" footer={<>Already registered? <Link to="/login">Sign in</Link></>}><form onSubmit={submit}><input type="email" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} required /><input type="password" placeholder="Password (at least 8 characters)" minLength="8" value={password} onChange={e => setPassword(e.target.value)} required /><button>Create account</button></form><Message text={message} /></AuthPage>;
}

function HomePage({ onSignOut }) {
  const navigate = useNavigate(); const [tickets, setTickets] = useState([]); const [title, setTitle] = useState(""); const [description, setDescription] = useState(""); const [message, setMessage] = useState("");
  async function loadTickets() { try { setTickets(await api("/tickets")); } catch (error) { setMessage(error.message); if (error.message.includes("token")) { onSignOut(); navigate("/login"); } } }
  useEffect(() => { loadTickets(); }, []);
  async function createTicket(event) { event.preventDefault(); setMessage(""); try { await api("/tickets", { method: "POST", body: JSON.stringify({ title, description }) }); setTitle(""); setDescription(""); loadTickets(); } catch (error) { setMessage(error.message); } }
  async function moveStatus(id, status) { setMessage(""); try { await api(`/tickets/${id}/status`, { method: "PATCH", body: JSON.stringify({ status }) }); loadTickets(); } catch (error) { setMessage(error.message); } }
  return <main className="home-page"><header><div><h1>My Tickets</h1><p>Create and track your support tickets.</p></div><button className="secondary" onClick={() => { onSignOut(); navigate("/login"); }}>Sign out</button></header><section className="create-ticket"><h2>New ticket</h2><form onSubmit={createTicket}><input placeholder="Ticket title" value={title} onChange={e => setTitle(e.target.value)} required /><textarea placeholder="Description (optional)" value={description} onChange={e => setDescription(e.target.value)} /><button>Create ticket</button></form></section><Message text={message} /><section className="ticket-list">{tickets.map(ticket => <article key={ticket.id}><div><h2>{ticket.title}</h2><p>{ticket.description || "No description"}</p><small>Status: <strong>{ticket.status}</strong></small></div>{ticket.status === "open" && <button onClick={() => moveStatus(ticket.id, "in_progress")}>Start progress</button>}{ticket.status === "in_progress" && <button onClick={() => moveStatus(ticket.id, "closed")}>Close ticket</button>}</article>)}{tickets.length === 0 && <p className="empty">No tickets yet. Create your first one above.</p>}</section></main>;
}

createRoot(document.getElementById("root")).render(<BrowserRouter><App /></BrowserRouter>);
