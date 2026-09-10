FROM node:24-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
ARG VITE_API_URL
# Render turns service environment variables into Docker build arguments.
# Local Docker Compose instead uses the value in frontend/.env.
RUN if [ -n "$VITE_API_URL" ]; then VITE_API_URL="$VITE_API_URL" npm run build; else npm run build; fi

FROM nginx:1.29-alpine
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
