import { useEffect, useState } from "react";
import { BrowserRouter, Link, Navigate, Route, Routes, useNavigate } from "react-router-dom";
import { createRoot } from "react-dom/client";
import "./styles.css";

// The backend API address is read from the frontend environment settings.
// This allows the frontend to communicate with the backend without
// hard-coding the server address directly into the application.
const API = import.meta.env.VITE_API_URL;

// Stop the application from starting if the backend address is missing.
if (!API) {
  throw new Error("VITE_API_URL must be set in frontend/.env");
}

// A common function used for all communication with the backend.
//
// It automatically:
// 1. Gets the user's login token, if they are logged in.
// 2. Sends the request to the backend.
// 3. Adds the login token to protected requests.
// 4. Reads the response from the backend.
// 5. Shows an error if the request was unsuccessful.
async function api(path, options = {}) {
  const token = localStorage.getItem("token");
  const response = await fetch(API + path, { ...options, headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}), ...options.headers } });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || "Request failed");
  return data;
}

// The main application component.
//
// It keeps track of whether the user is currently logged in
// and decides which page should be displayed based on the URL.
function App() {
  // The login token is saved in the browser so the user can
  // remain logged in when navigating between pages.
  const [token, setToken] = useState(localStorage.getItem("token"));

  // Removes the login token and marks the user as logged out.
  const signOut = () => { localStorage.removeItem("token"); setToken(null); };

  // Define all pages and the URLs where they can be accessed.
  return <Routes>
    // Login page for existing users.
    <Route path="/login" element={<LoginPage token={token} onLogin={setToken} />} />

    // Registration page for new users.
    <Route path="/register" element={<RegisterPage token={token} />} />

    // Main ticket page.
    // ProtectedRoute makes sure only logged-in users can access it.
    <Route path="/" element={<ProtectedRoute token={token}><HomePage onSignOut={signOut} /></ProtectedRoute>} />

    // If the user enters an unknown URL, send them either
    // to the home page if logged in or to the login page.
    <Route path="*" element={<Navigate to={token ? "/" : "/login"} replace />} />
  </Routes>;
}

// Prevents users who are not logged in from accessing protected pages.
// If a login token exists, the requested page is displayed.
// Otherwise, the user is sent to the login page.
function ProtectedRoute({ token, children }) { return token ? children : <Navigate to="/login" replace />; }

// Provides a shared layout for the login and registration pages.
// This keeps the design and structure of both pages consistent.
function AuthPage({ title, children, footer }) { return <main className="auth-page"><section className="card"><h1>Ticket System</h1><h2>{title}</h2>{children}<p className="auth-footer">{footer}</p></section></main>; }

// Displays an error or information message when text is provided.
// If there is no message, nothing is displayed.
function Message({ text }) { return text ? <p className="message">{text}</p> : null; }

// Handles the user login process.
function LoginPage({ token, onLogin }) {
  const navigate = useNavigate(); const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [message, setMessage] = useState("");

  // If the user is already logged in, there is no need to show
  // the login form, so they are sent directly to the home page.
  if (token) return <Navigate to="/" replace />;

  // Sends the entered email and password to the backend.
  async function submit(event) {
    event.preventDefault(); setMessage("");

    try {
      // Ask the backend to verify the user's login details.
      const data = await api("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });

      // Save the login token in the browser so it can be used
      // for future requests that require authentication.
      localStorage.setItem("token", data.token);

      // Tell the application that the user is now logged in.
      onLogin(data.token);

      // Take the user to their ticket dashboard.
      navigate("/");
    }
    catch (error) {
      // Show the backend's error message if login fails.
      setMessage(error.message);
    }
  }

  return <AuthPage title="Sign in" footer={<>New here? <Link to="/register">Create an account</Link></>}><form onSubmit={submit}><input type="email" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} required /><input type="password" placeholder="Password" minLength="8" value={password} onChange={e => setPassword(e.target.value)} required /><button>Sign in</button></form><Message text={message} /></AuthPage>;
}

// Handles creation of a new user account.
function RegisterPage({ token }) {
  const navigate = useNavigate(); const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [message, setMessage] = useState("");

  // Logged-in users do not need to register again.
  if (token) return <Navigate to="/" replace />;

  // Sends the new user's details to the backend.
  async function submit(event) {
    event.preventDefault(); setMessage("");

    try {
      // Ask the backend to create the new account.
      await api("/auth/register", { method: "POST", body: JSON.stringify({ email, password }) });

      // After successful registration, send the user to the login page.
      navigate("/login");
    }
    catch (error) {
      // Show an error message if account creation fails.
      setMessage(error.message);
    }
  }

  return <AuthPage title="Create account" footer={<>Already registered? <Link to="/login">Sign in</Link></>}><form onSubmit={submit}><input type="email" placeholder="Email" value={email} onChange={e => setEmail(e.target.value)} required /><input type="password" placeholder="Password (at least 8 characters)" minLength="8" value={password} onChange={e => setPassword(e.target.value)} required /><button>Create account</button></form><Message text={message} /></AuthPage>;
}

// Main page where logged-in users can create and manage their tickets.
function HomePage({ onSignOut }) {
  const navigate = useNavigate();

  // Store the user's tickets and the information entered
  // into the new-ticket form.
  const [tickets, setTickets] = useState([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [message, setMessage] = useState("");

  // Retrieves all tickets belonging to the logged-in user.
  async function loadTickets() {
    try {
      // Ask the backend for the user's tickets and display them.
      setTickets(await api("/tickets"));
    }
    catch (error) {
      // Display any error returned by the backend.
      setMessage(error.message);

      // If the login token is invalid or expired, log the user out
      // and send them back to the login page.
      if (error.message.includes("token")) {
        onSignOut();
        navigate("/login");
      }
    }
  }

  // Load the user's tickets automatically when the home page
  // is opened for the first time.
  useEffect(() => { loadTickets(); }, []);

  // Creates a new support ticket using the information
  // entered in the form.
  async function createTicket(event) {
    event.preventDefault();
    setMessage("");

    try {
      // Send the new ticket information to the backend.
      await api("/tickets", { method: "POST", body: JSON.stringify({ title, description }) });

      // Clear the form after the ticket has been created.
      setTitle("");
      setDescription("");

      // Refresh the ticket list so the new ticket appears immediately.
      loadTickets();
    }
    catch (error) {
      // Display an error if the ticket could not be created.
      setMessage(error.message);
    }
  }

  // Changes a ticket from one status to another.
  // For example, an "open" ticket can be moved to "in_progress".
  async function moveStatus(id, status) {
    setMessage("");

    try {
      // Ask the backend to update the ticket's status.
      await api(`/tickets/${id}/status`, { method: "PATCH", body: JSON.stringify({ status }) });

      // Refresh the list so the updated status is immediately visible.
      loadTickets();
    }
    catch (error) {
      // Display an error if the status could not be changed.
      setMessage(error.message);
    }
  }

  return <main className="home-page"><header><div><h1>My Tickets</h1><p>Create and track your support tickets.</p></div><button className="secondary" onClick={() => { onSignOut(); navigate("/login"); }}>Sign out</button></header><section className="create-ticket"><h2>New ticket</h2><form onSubmit={createTicket}><input placeholder="Ticket title" value={title} onChange={e => setTitle(e.target.value)} required /><textarea placeholder="Description (optional)" value={description} onChange={e => setDescription(e.target.value)} /><button>Create ticket</button></form></section><Message text={message} /><section className="ticket-list">{tickets.map(ticket => <article key={ticket.id}><div><h2>{ticket.title}</h2><p>{ticket.description || "No description"}</p><small>Status: <strong>{ticket.status}</strong></small></div>{ticket.status === "open" && <button onClick={() => moveStatus(ticket.id, "in_progress")}>Start progress</button>}{ticket.status === "in_progress" && <button onClick={() => moveStatus(ticket.id, "closed")}>Close ticket</button>}</article>)}{tickets.length === 0 && <p className="empty">No tickets yet. Create your first one above.</p>}</section></main>;
}

// Start the React application and make the routing system
// available to all pages.
createRoot(document.getElementById("root")).render(<BrowserRouter><App /></BrowserRouter>);