# Ticket System

A deliberately small ticket system with a Gin backend and React frontend. The backend uses SQLite's in-memory database, so data resets when it restarts.

## Project structure

```
backend/   Go, Gin, SQLite, Docker
frontend/  React and React Router client
```

## Run the API

```powershell
cd backend
Copy-Item .env.example .env
# Edit .env and set JWT_SECRET before the first run.
go run .
```

It listens on `http://localhost:8080`. Check it with `curl http://localhost:8080/health`.

## Run with Docker

To run the backend only (the assignment's Docker flow), run these commands:

```bash
cd backend
cp .env.example .env # Set JWT_SECRET in .env first.
docker build -t ticket-system .
docker run --rm --env-file .env -p 8080:8080 ticket-system
curl http://localhost:8080/health
```

To run both the backend and frontend together:

```bash
cp backend/.env.example backend/.env  # Set JWT_SECRET in this file.
cp frontend/.env.example frontend/.env
docker compose up --build
```

Then open the frontend at `http://localhost:3000`. The API health check is at `http://localhost:8080/health`.

To stop both containers, press `Ctrl+C`, or run `docker compose down` in another terminal.

## Run the React client

```powershell
cd frontend
npm.cmd install
Copy-Item .env.example .env
npm.cmd run dev
```

For a deployed API, use its URL instead.

## Deploy on Render with Docker

Deploy the two folders as two **Docker Web Services** from the same GitHub repository. Do not deploy `docker-compose.yml` directly; Render builds each service from its own Dockerfile.

### 1. Deploy the backend

Create a new **Web Service** and set:

| Render field | Value |
| --- | --- |
| Language | Docker |
| Root Directory | `backend` |
| Dockerfile Path | `Dockerfile` |
| Docker Build Context Directory | `.` |
| Health Check Path | `/health` |

Set these environment variables in Render (not in Git):

```text
PORT=10000
JWT_SECRET=a-long-random-secret
```

Deploy it, open its public URL, and confirm `<backend-url>/health` returns `{"status":"ok"}`.

### 2. Deploy the frontend

Create another **Web Service** and set:

| Render field | Value |
| --- | --- |
| Language | Docker |
| Root Directory | `frontend` |
| Dockerfile Path | `Dockerfile` |
| Docker Build Context Directory | `.` |

Set these Render environment variables:

```text
PORT=10000
VITE_API_URL=https://your-backend-name.onrender.com
```

Deploy the frontend. Render makes `VITE_API_URL` available as a Docker build argument, so it is embedded in the React build. Redeploy the frontend whenever that API URL changes.

## API

`POST /auth/register` and `POST /auth/login` accept `{"email":"user@example.com","password":"password"}`. Login returns `{"token":"..."}`. All ticket routes require `Authorization: Bearer <token>`.

| Method | Path | Body |
| --- | --- | --- |
| GET | `/health` | — |
| POST | `/auth/register` | email, password |
| POST | `/auth/login` | email, password |
| POST | `/tickets` | title, description (optional) |
| GET | `/tickets` | — |
| GET | `/tickets/{id}` | — |
| PATCH | `/tickets/{id}/status` | status |

Ticket statuses can only move from `open` to `in_progress`, then from `in_progress` to `closed`. A ticket is never returned or changed for another user.

## Deployment

The Dockerfile is ready for any Docker-compatible free host (for example Render, Railway, or Fly.io). Set `JWT_SECRET` in the host's environment settings and publish port `8080`. Add the deployed API URL and public `/health` URL here before submission.
