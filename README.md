# Ticket Management System

A small ticket-management application with a React frontend and a Go (Gin) backend. You can create an account, sign in, create tickets, and move your own tickets through their allowed statuses.

## Try the live application

- **Frontend:** [Open Ticket Management System](https://ticket-management-system-1-y5ah.onrender.com/)
- **Backend health check:** [Open API health check](https://ticket-management-system-6uhd.onrender.com/health)

The live backend uses temporary in-memory SQLite storage. Its data is cleared whenever the backend restarts.

## What you need to run it locally

1. Install [Docker Desktop](https://www.docker.com/products/docker-desktop/).
2. Start Docker Desktop and wait until it says Docker is running.
3. Download or clone this project.

You do **not** need to install Go, Node.js, or npm when using Docker.

## Run the complete project with Docker

Open a terminal in the main project folder (the folder containing `docker-compose.yml`). Before running Docker, manually create these two files. Do not commit them to Git.

### 1. Create `backend/.env`

Create a new file named exactly `.env` inside the `backend` folder. Paste this into it:

```env
PORT=8080
JWT_SECRET=replace-this-with-a-long-random-secret
```

Replace the `JWT_SECRET` value with a long private value of your choice. Do not share it publicly.

### 2. Create `frontend/.env`

Create a new file named exactly `.env` inside the `frontend` folder. Paste this into it:

```env
VITE_API_URL=http://localhost:8080
PORT=80
```

Both lines are required. `PORT=80` is the port used **inside** the frontend container; you still open the website at `http://localhost:3000` because Docker maps port 3000 on your computer to port 80 in the container.

> On Windows, make sure the files are named `.env`, not `.env.txt`. If File Explorer hides extensions, enable **View → File name extensions** before creating them.

### 3. Start the application

In the main project folder, run:

```bash
docker compose up --build
```

The first run can take a few minutes because Docker downloads the required images.

When the log messages stop showing errors, open:

- Frontend: [http://localhost:3000](http://localhost:3000)
- Backend health check: [http://localhost:8080/health](http://localhost:8080/health)

You should see this health response:

```json
{"status":"ok"}
```

### Stop the application

Press `Ctrl+C` in the terminal that is running Docker. To remove the stopped containers, run:

```bash
docker compose down
```

### If the frontend exits with `host not found in "${PORT}"`

Open `frontend/.env` and confirm it contains this line:

```env
PORT=80
```

Then restart the containers:

```bash
docker compose down
docker compose up
```

## Run only the backend API with Docker

Use this when you only want to test the assignment API without the React website.

```bash
cd backend
docker build -t ticket-system .
docker run --rm --env-file .env -p 8080:8080 ticket-system
```

Then visit [http://localhost:8080/health](http://localhost:8080/health).

## Features

- Register and sign in with email and password
- Passwords are stored securely as bcrypt hashes
- JWT-protected ticket APIs
- Users can see and update only their own tickets
- Ticket status flow: `open` → `in_progress` → `closed`
- Closed tickets cannot be reopened

## API reference

All ticket endpoints require this request header after sign-in:

```text
Authorization: Bearer <token>
```

| Method | Endpoint | Purpose |
| --- | --- | --- |
| GET | `/health` | Check whether the backend is running |
| POST | `/auth/register` | Create a user account |
| POST | `/auth/login` | Sign in and receive a JWT token |
| POST | `/tickets` | Create a ticket |
| GET | `/tickets` | List the signed-in user's tickets |
| GET | `/tickets/{id}` | Get one of the signed-in user's tickets |
| PATCH | `/tickets/{id}/status` | Change a ticket's status |

## Project structure

```text
backend/   Go, Gin, SQLite, Docker
frontend/  React, React Router, Nginx, Docker
```
